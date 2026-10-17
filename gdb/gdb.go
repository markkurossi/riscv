//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package gdb implements GDB remote debugging protocol and debugging
// stub.
package gdb

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/markkurossi/riscv/cpu"
	"github.com/markkurossi/riscv/mmu"
)

// Conn implements debugger connection.
type Conn struct {
	stub *Stub
	c    net.Conn
	r    *bufio.Reader
}

// State defines debugging stub state.
type State int

// Debugging stub states.
const (
	Start State = iota
	Run
)

// Stub implementes GDB remote debugging stub.
type Stub struct {
	m             sync.Mutex
	cond          *sync.Cond
	listener      net.Listener
	cpu           *cpu.CPU
	conn          *Conn
	state         State
	signal        int
	fErrorMessage bool
}

// New creates a new GDB remote debugging stub for the cpu and listen
// address.
func New(cpu *cpu.CPU, listen string) (*Stub, error) {
	var proto, addr string

	idx := strings.IndexByte(listen, ':')
	if idx < 0 {
		return nil, fmt.Errorf("invalid address: %v", listen)
	} else if idx == 0 {
		proto = "tcp"
		addr = listen
	} else {
		proto = listen[:idx]
		addr = listen[idx+1:]
	}

	l, err := net.Listen(proto, addr)
	if err != nil {
		return nil, err
	}

	stub := &Stub{
		listener: l,
		cpu:      cpu,
		state:    Start,
		signal:   5,
	}
	stub.cond = sync.NewCond(&stub.m)

	return stub, nil
}

// Run listens for debugger connections. Only one connection is
// allowed at a time.
func (stub *Stub) Run() {
	for {
		conn, err := stub.listener.Accept()
		if err != nil {
			log.Printf("listen: %v", err)
			continue
		}

		c := &Conn{
			stub: stub,
			c:    conn,
			r:    bufio.NewReader(conn),
		}

		stub.setConnection(c)

		stub.m.Lock()
		for stub.conn != nil {
			stub.cond.Wait()
		}
		stub.m.Unlock()
	}
}

func (stub *Stub) setConnection(conn *Conn) {
	stub.m.Lock()
	defer stub.m.Unlock()

	stub.conn = conn
	stub.cond.Broadcast()
}

// Breakpoint implements GDB.Breakpoint.
func (stub *Stub) Breakpoint() {
	first := true
	stub.m.Lock()
	for stub.conn == nil {
		if first {
			fmt.Printf("waiting for debugger...\n")
			first = false
		}
		stub.cond.Wait()
	}
	conn := stub.conn

	stub.signal = 5

	stub.m.Unlock()
	fmt.Printf("debugger connected\n")

	err := conn.process()
	if err != nil {
		log.Printf("process: %v", err)
		conn.c.Close()
		stub.setConnection(nil)
	}
}

var wellKnown = map[string]string{
	"Hg0":                "OK",
	"qAttached":          "1",
	"qC":                 "QC1",
	"qSymbol::":          "OK",
	"qThreadExtraInfo,1": fmt.Sprintf("%x", []byte("RISC-V hart 0")),
	"qfThreadInfo":       "m1", // Thread list contains thread ID 1.
	"qsThreadInfo":       "l",  // No more thread IDs to report.
	"vCont?":             "vCont;c;s",

	"qExecAndArgs":    "",
	"qOffsets":        "",
	"qTStatus":        "",
	"vMustReplyEmpty": "",
}

func (conn *Conn) process() error {
	switch conn.stub.state {
	case Start:
		// GDB starts with '+'
		b, err := conn.r.ReadByte()
		if err != nil {
			return err
		}
		if b != '+' {
			return fmt.Errorf("rsp error: expected initial '+'")
		}
		conn.stub.state = Run

	case Run:
		err := conn.handleLastSignal()
		if err != nil {
			return err
		}
	}

	for {
		cmd, err := conn.readCommand()
		if err != nil {
			return err
		}

		// A stub is required to support the `g', `G', `m', `M', `c',
		// and `s' commands. All other commands are optional.
		v, ok := wellKnown[cmd]
		if ok {
			err = conn.response(v)
		} else {
			switch {
			case cmd == "":
				err = conn.response("")

			case cmd == "?":
				err = conn.handleLastSignal()

			case cmd == "c":
				return nil

			case cmd == "g":
				err = conn.readGeneralRegisters()

			case strings.HasPrefix(cmd, "m"):
				err = conn.readMemory(cmd[1:])

			case strings.HasPrefix(cmd, "qSupported:"):
				err = conn.handleSupported(cmd[11:])

			default:
				fmt.Printf("unsupported: %v\n", cmd)
				err = conn.response("")
			}
		}
		if err != nil {
			return err
		}
	}
}

func (conn *Conn) handleLastSignal() error {
	return conn.response(fmt.Sprintf("S%02X", conn.stub.signal))
}

func (conn *Conn) readGeneralRegisters() error {
	var result string

	for i := 0; i < len(conn.stub.cpu.X); i++ {
		result += encodeUint64(conn.stub.cpu.X[i])
	}
	result += encodeUint64(conn.stub.cpu.PC)

	return conn.response(result)
}

func (conn *Conn) readMemory(cmd string) error {
	addrStr, lengthStr, ok := strings.Cut(cmd, ",")
	if !ok {
		return fmt.Errorf("rsp error: invalid m%v command", cmd)
	}
	addr, err := strconv.ParseUint(addrStr, 16, 64)
	if err != nil {
		return err
	}
	length, err := strconv.ParseUint(lengthStr, 16, 64)
	if err != nil {
		return err
	}

	mem := conn.stub.cpu.MMU

	var result string

	if mem.Satp() == mmu.SatpModeBare {
		// MMU off, check that the address is valid.
		if mem.Mem.Contains(addr) && mem.Mem.Contains(addr+length-1) {
			offset := mem.Mem.Offset(addr)
			result = fmt.Sprintf("%x", mem.Mem.RAM[offset:offset+length])
		} else if mem.MMIO.Contains(addr, length) {
			for i := uint64(0); i < length; i++ {
				b, err := mem.MMIO.Load8(addr + i)
				if err != nil {
					break
				}
				result += fmt.Sprintf("%02x", b)
			}
		} else {
			fmt.Printf("invalid physical memory read %x\n", addr)
			return conn.response("E01")
		}
	} else {
		// MMU on.
		for i := uint64(0); i < length; i++ {
			b, err := mem.Load8(addr + i)
			if err != nil {
				if i == 0 {
					fmt.Printf("invalid virtual memory read %x\n", addr+i)
					return conn.response("E01")
				}
				break
			}
			result += fmt.Sprintf("%02x", b)
		}
	}

	return conn.response(result)
}

func encodeUint64(v uint64) string {
	var buf [8]byte

	binary.LittleEndian.PutUint64(buf[:], v)
	return fmt.Sprintf("%x", buf[:])
}

func (conn *Conn) handleSupported(cmd string) error {
	response := []string{
		"PacketSize=4000",
	}
	for _, f := range strings.Split(cmd, ";") {
		switch f {
		case "swbreak+":
		case "hwbreak+":
		case "vContSupported+":

		case "error-message+":
			conn.stub.fErrorMessage = true

		default:
			if false {
				fmt.Printf("qSupported: %v: false\n", f)
			}
		}
	}
	return conn.response(strings.Join(response, ";"))
}

func (conn *Conn) readCommand() (string, error) {
	var buf bytes.Buffer

	b, err := conn.r.ReadByte()
	if err != nil {
		return "", err
	}
	if b != '$' {
		return "", fmt.Errorf("rsp error: got '%c', expected '$'", b)
	}
	var computed uint8
	for {
		b, err = conn.r.ReadByte()
		if err != nil {
			return "", err
		}
		if b == '#' {
			break
		}
		buf.WriteByte(b)
		computed += b
	}

	// Read checksum.
	var checksum uint8
	for i := 0; i < 2; i++ {
		v, err := conn.readHexDigit()
		if err != nil {
			return "", err
		}
		checksum <<= 4
		checksum += v
	}

	if false {
		fmt.Printf("cmd: %v, sum=%02x[%02x]\n", buf.String(),
			computed, checksum)
	}

	_, err = conn.c.Write([]byte{'+'})
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (conn *Conn) readHexDigit() (uint8, error) {
	b, err := conn.r.ReadByte()
	if err != nil {
		return 0, err
	}
	if '0' <= b && b <= '9' {
		return b - '0', nil
	}
	if 'a' <= b && b <= 'f' {
		return b - 'a' + 10, nil
	}
	if 'A' <= b && b <= 'F' {
		return b - 'A' + 10, nil
	}
	return 0, fmt.Errorf("invalid hex digit %c", b)
}

func (conn *Conn) response(r string) error {
	var sum uint8

	response := []byte{'$'}
	for _, b := range []byte(r) {
		sum += b
		response = append(response, b)
	}
	response = append(response, '#')

	checksum := fmt.Sprintf("%02x", sum)
	response = append(response, []byte(checksum)...)

	if false {
		fmt.Printf("response:\n%s", hex.Dump(response))
	}

	for {
		_, err := conn.c.Write(response)
		if err != nil {
			return err
		}
		b, err := conn.r.ReadByte()
		if err != nil {
			return err
		}
		switch b {
		case '+':
			return nil
		case '-':
			// Retransmit
		default:
			return fmt.Errorf("rsp error: invalid acknowledgment %c", b)
		}
	}
}

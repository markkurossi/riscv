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
	kernelEntry   uint64
	kernelDelta   uint64
	conn          *Conn
	state         State
	signal        int
	fErrorMessage bool

	bps map[uint64]*Breakpoint
}

type Breakpoint struct {
	vaddr uint64
	paddr uint64
	code  []byte
}

// New creates a new GDB remote debugging stub for the cpu and listen
// address.
func New(cpu *cpu.CPU, kernelEntry uint64, listen string) (*Stub, error) {
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
		listener:    l,
		cpu:         cpu,
		kernelEntry: kernelEntry,
		state:       Start,
		signal:      5,
		bps:         make(map[uint64]*Breakpoint),
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

func (stub *Stub) breakpoint() {
	first := true
	stub.m.Lock()
	for stub.conn == nil {
		if first {
			fmt.Printf("gdb: waiting for debugger...\n")
			first = false
		}
		stub.cond.Wait()
	}
	conn := stub.conn

	stub.signal = 5

	stub.m.Unlock()
	fmt.Printf("gdb: debugger connected\n")

	err := conn.process()
	if err != nil {
		log.Printf("process: %v", err)
		conn.c.Close()
		stub.setConnection(nil)
	}
}

// Start implements GDB.Start.
func (stub *Stub) Start() {
	stub.breakpoint()
}

// Breakpoint implements GDB.Breakpoint.
func (stub *Stub) Breakpoint() {
	// Check if this is our breakpoint.
	pc := stub.cpu.PC
	found := false

	stub.m.Lock()
	for _, bp := range stub.bps {
		if bp.vaddr == pc || bp.paddr == pc {
			found = true
			break
		}
	}
	stub.m.Unlock()
	if !found {
		fmt.Printf("gdb: not our breakpoint\n")
		return
	}

	stub.breakpoint()
}

// unsupported: Mffffffff806941b2,2:5d71
// unsupported: vKill;a410
// unsupported: k

var wellKnown = map[string]string{
	"Hg0":       "OK",
	"qAttached": "1",
	"qC":        "QC1",
	// "qSymbol::":          "OK",
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

			case cmd == "k":
				fmt.Printf("gdb: kill: terminating\n")
				conn.stub.cpu.Shutdown()
				_ = conn.response("OK")
				return nil

			case strings.HasPrefix(cmd, "m"):
				err = conn.readMemory(cmd[1:])

			case strings.HasPrefix(cmd, "vKill;"):
				err = conn.response("")

			case strings.HasPrefix(cmd, "qSupported:"):
				err = conn.handleSupported(cmd[11:])

			case cmd == "qSymbol::":
				err = conn.resolveSymbols()

			case strings.HasPrefix(cmd, "z"):
				err = conn.deleteBreak(cmd)

			case strings.HasPrefix(cmd, "Z"):
				// Zt,addr,length: t is type:
				//   `0' - software breakpoint,
				//   `1' - hardware breakpoint,
				//   `2' - write watchpoint,
				//   `3' - read watchpoint,
				//   `4' - access watchpoint;
				//
				// addr is address; length is in bytes. For a software
				// breakpoint, length specifies the size of the
				// instruction to be patched. For hardware breakpoints
				// and watchpoints length specifies the memory region
				// to be monitored. To avoid potential problems with
				// duplicate packets, the operations should be
				// implemented in an idempotent way.
				err = conn.insertBreak(cmd)

			default:
				fmt.Printf("gdb: unsupported: %v\n", cmd)
				err = conn.response("")
			}
		}
		if err != nil {
			return err
		}
	}
}

var symbols = []string{"_start", "not_here"}

func (conn *Conn) resolveSymbols() error {
	fmt.Printf("gdb: resolving symbols\n")
	for _, sym := range symbols {
		q := fmt.Sprintf("qSymbol:%x", []byte(sym))
		err := conn.response(q)
		if err != nil {
			return err
		}
		r, err := conn.readCommand()
		if err != nil {
			return err
		}
		parts := strings.Split(r, ":")
		if len(parts) != 3 {
			fmt.Printf("gdb: ignoring invalid qSymbol response %v\n", r)
			continue
		}
		if len(parts[1]) == 0 {
			fmt.Printf("gdb: symbol %v not found\n", sym)
			continue
		}
		addr, err := strconv.ParseUint(parts[1], 16, 64)
		if err != nil {
			fmt.Printf("gdb: invalid symbol address: %v\n", err)
			continue
		}
		fmt.Printf(" - %v => %x\n", sym, addr)
		switch sym {
		case "_start":
			conn.stub.kernelDelta = addr - conn.stub.kernelEntry
			fmt.Printf(" - \u0394: %x\n", conn.stub.kernelDelta)
		}
	}
	return conn.response("OK")
}

func (conn *Conn) deleteBreak(cmd string) error {
	parts := strings.Split(cmd, ",")
	if len(parts) != 3 || parts[0] != "z0" {
		fmt.Printf("gdb: breakpoint %v not supported\n", cmd)
		return conn.response("")
	}

	addr, err := strconv.ParseUint(parts[1], 16, 64)
	if err != nil {
		fmt.Printf("gdb: invalid address: %v\n", err)
		return conn.response("")
	}
	length, err := strconv.ParseUint(parts[2], 16, 64)
	if err != nil {
		fmt.Printf("gdb: invalid length: %v\n", err)
		return conn.response("")
	}

	bp, ok := conn.stub.bps[addr]
	if !ok || length != uint64(len(bp.code)) {
		fmt.Printf("unknown breakpoint %x\n", addr)
		return conn.response("E01")
	}

	err = conn.write(addr, bp.code)
	if err != nil {
		fmt.Printf("write failed: %v\n", err)
		return conn.response("E01")
	}

	fmt.Printf("%v: deleted breakpoint with %x\n", cmd, bp.code)

	delete(conn.stub.bps, addr)

	return conn.response("OK")
}

func (conn *Conn) insertBreak(cmd string) error {
	parts := strings.Split(cmd, ",")
	if len(parts) != 3 || parts[0] != "Z0" {
		fmt.Printf("breakpoint %v not supported\n", cmd)
		return conn.response("")
	}

	addr, err := strconv.ParseUint(parts[1], 16, 64)
	if err != nil {
		fmt.Printf("invalid address: %v\n", err)
		return conn.response("")
	}
	length, err := strconv.ParseUint(parts[2], 16, 64)
	if err != nil {
		fmt.Printf("invalid length: %v\n", err)
		return conn.response("")
	}

	code, err := conn.read(addr, length)
	if err != nil {
		fmt.Printf("read failed: %v\n", err)
		return conn.response("")
	}

	bp := &Breakpoint{
		vaddr: addr,
		code:  code,
	}

	switch length {
	case 2:
		// c.ebreak: 0x0902
		err = conn.write(addr, []byte{0x02, 0x09})

	case 4:
		// ebreak: 0x00100073
		err = conn.write(addr, []byte{0x73, 0x00, 0x10, 0x00})

	default:
		err = fmt.Errorf("invalid instruction length %v", length)
	}
	if err != nil {
		fmt.Printf("write failed: %v\n", err)
		return conn.response("")
	}

	fmt.Printf("%v: inserted breakpoint to replace %x\n", cmd, code)

	conn.stub.bps[addr] = bp

	return conn.response("OK")
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

	data, err := conn.read(addr, length)
	if err != nil {
		fmt.Printf("read failed: %v\n", err)
		return conn.response("E01")
	}

	return conn.response(fmt.Sprintf("%x", data))
}

func (conn *Conn) read(addr, length uint64) ([]byte, error) {
	mem := conn.stub.cpu.MMU

	var result []byte

	if mem.Satp() == mmu.SatpModeBare {
		// MMU off. Check if it is an upper virtual address.
		if addr >= mem.Mem.RAMEnd && addr > conn.stub.kernelDelta {
			addr -= conn.stub.kernelDelta
			fmt.Printf("read: mapped virtual %x to %x\n",
				addr+conn.stub.kernelDelta, addr)
		}

		// MMU off, check that the address is valid.
		if mem.Mem.Contains(addr) && mem.Mem.Contains(addr+length-1) {
			offset := mem.Mem.Offset(addr)
			result = make([]byte, length)
			copy(result, mem.Mem.RAM[offset:offset+length])

			fmt.Printf("read, memory at %x:\n%s", addr,
				hex.Dump(mem.Mem.RAM[offset:offset+256]))

		} else if mem.MMIO.Contains(addr, length) {
			for i := uint64(0); i < length; i++ {
				b, err := mem.MMIO.Load8(addr + i)
				if err != nil {
					break
				}
				result = append(result, b)
			}
		} else {
			return nil, fmt.Errorf("invalid physical address %x", addr)
		}
	} else {
		// MMU on.
		for i := uint64(0); i < length; i++ {
			b, err := mem.Load8(addr + i)
			if err != nil {
				if i == 0 {
					return nil,
						fmt.Errorf("invalid virtual address %x", addr+i)
				}
				break
			}
			result = append(result, b)
		}
	}

	return result, nil
}

func (conn *Conn) write(addr uint64, data []byte) error {
	mem := conn.stub.cpu.MMU

	length := uint64(len(data))

	if mem.Satp() == mmu.SatpModeBare {
		// MMU off. Check if it is an upper virtual address.
		if addr >= mem.Mem.RAMEnd && addr > conn.stub.kernelDelta {
			addr -= conn.stub.kernelDelta
			fmt.Printf("write: mapped virtual %x to %x\n",
				addr+conn.stub.kernelDelta, addr)
		}

		// MMU off, check that the address is valid.
		if mem.Mem.Contains(addr) && mem.Mem.Contains(addr+length-1) {
			offset := mem.Mem.Offset(addr)
			copy(mem.Mem.RAM[offset:offset+length], data)
		} else if mem.MMIO.Contains(addr, length) {
			for i, b := range data {
				err := mem.MMIO.Store8(addr+uint64(i), b)
				if err != nil {
					break
				}
			}
		} else {
			return fmt.Errorf("invalid physical address %x", addr)
		}
	} else {
		// MMU on.
		for i, b := range data {
			err := mem.Store8(addr+uint64(i), b)
			if err != nil {
				return err
			}
		}
	}

	return nil
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

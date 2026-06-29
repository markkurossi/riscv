//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

//lint:file-ignore ST1003 to match the CSR naming conventions.

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"time"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/mmu"
)

const (
	debugCSR = false
)

type CSR int

func (csr CSR) String() string {
	return csrName(int(csr))
}

func (csr CSR) ReadWrite() bool {
	return csr>>10&0b11 != 0b11
}

func (csr CSR) ReadOnly() bool {
	return csr>>10&0b11 == 0b11
}

func (csr CSR) Privilege() isa.PrivilegeMode {
	return isa.PrivilegeMode(csr >> 8 & 0b11)
}

const (
	CsrFcsr          = 0x003
	CsrVstart        = 0x008
	CsrVxsat         = 0x009
	CsrVxrm          = 0x00a
	CsrVcsr          = 0x00f
	CsrSeed          = 0x015
	CsrSstatus       = 0x100
	CsrSie           = 0x104
	CsrStvec         = 0x105
	CsrScounteren    = 0x106
	CsrSscratch      = 0x140
	CsrSepc          = 0x141
	CsrScause        = 0x142
	CsrStval         = 0x143
	CsrSip           = 0x144
	CsrStimecmp      = 0x14d // RISC-V Sstc (Supervisor Time Compare) extension
	CsrSatp          = 0x180
	CsrMstatus       = 0x300
	CsrMisa          = 0x301
	CsrMedeleg       = 0x302
	CsrMideleg       = 0x303
	CsrMie           = 0x304
	CsrMtvec         = 0x305
	CsrMcounteren    = 0x306
	CsrMenvcfg       = 0x30a
	CsrMstateen0     = 0x30c
	CsrMcountinhibit = 0x320
	CsrMhpmevent3    = 0x321
	CsrMscratch      = 0x340
	CsrMepc          = 0x341
	CsrMcause        = 0x342
	CsrMtval         = 0x343
	CsrMip           = 0x344
	CsrMpmpcfg0      = 0x3a0
	CsrMpmpcfg2      = 0x3b0
	CsrMpmpaddr8     = 0x3b1
	CsrTselect       = 0x7a0
	CsrTdata1        = 0x7a1
	CsrTinfo         = 0x7a4
	CsrCycle         = 0xc00
	CsrTime          = 0xc01
	CsrInstret       = 0xc02
	CsrVl            = 0xc20
	CsrVtype         = 0xc21
	CsrVlenb         = 0xc22
	CsrSenvcfg       = 0xda0
	CsrMvendorid     = 0xf11
	CsrMarchid       = 0xf12
	CsrMimpid        = 0xf13
	CsrMhartid       = 0xf14
	CsrScountinhibit = 0xfb0

	CsrGoemuDebug      = 0x7c0
	CsrGoemuTime       = 0x7c1
	CsrGoemuCPUProfile = 0x7c2
)

var csrs = map[int]string{
	// Unprivileged CSR addresses.

	// Unprivileged Floating-Point CSRs.
	0x001: "fflags", // Floating-Point Accrued Exceptions.
	0x002: "frm",    // Floating-Point Dynamic Rounding Mode.
	0x003: "fcsr",   // Floating-Point Control and Status Register.

	// Unprivileged Vector CSRs.
	0x008: "vstart", // Vector start position.
	0x009: "vxsat",  // Fixed-point accrued saturation flag.
	0x00a: "vxrm",   // Fixed-point rounding mode.
	0x00f: "vcsr",   // Vector control and status register.
	0xc20: "vl",     // Vector length.
	0xc21: "vtype",  // Vector data type register.
	0xc22: "vlenb",  // Vector register length in bytes.

	// Unprivileged Zicfiss extension CSR.
	0x011: "ssp", // Shadow Stack Pointer.

	// Unprivileged Entropy Source Extension CSR.
	0x015: "seed", // Seed for cryptographic random bit generators.

	// Unprivileged Zcmt Extension CSR.
	0x017: "jvt", // Table jump base vector and control register.

	// Unprivileged Counter/Timers.
	0xC00: "cycle",       // Cycle counter for RDCYCLE instruction.
	0xC01: "time",        // Timer for RDTIME instruction.
	0xC02: "instret",     // Instructions-retired counter for RDINSTRET instr.
	0xC03: "hpmcounter3", // Performance-monitoring counters.
	0xC04: "hpmcounter4",
	0xC05: "hpmcounter5",
	0xC06: "hpmcounter6",
	0xC07: "hpmcounter7",
	0xC08: "hpmcounter8",
	0xC09: "hpmcounter9",
	0xC0a: "hpmcounter10",
	0xC0b: "hpmcounter11",
	0xC0c: "hpmcounter12",
	0xC0d: "hpmcounter13",
	0xC0e: "hpmcounter14",
	0xC0f: "hpmcounter15",
	0xC10: "hpmcounter16",
	0xC11: "hpmcounter17",
	0xC12: "hpmcounter18",
	0xC13: "hpmcounter19",
	0xC14: "hpmcounter20",
	0xC15: "hpmcounter21",
	0xC16: "hpmcounter22",
	0xC17: "hpmcounter23",
	0xC18: "hpmcounter24",
	0xC19: "hpmcounter25",
	0xC1a: "hpmcounter26",
	0xC1b: "hpmcounter27",
	0xC1c: "hpmcounter28",
	0xC1d: "hpmcounter29",
	0xC1e: "hpmcounter30",
	0xC1f: "hpmcounter31",

	// Supervisor-level CSR addresses.

	// Supervisor Trap Setup.
	0x100: "sstatus",    // Supervisor status register.
	0x104: "sie",        // Supervisor interrupt-enable register.
	0x105: "stvec",      // Supervisor trap handler base address.
	0x106: "scounteren", // Supervisor counter enable.

	// Supervisor Configuration.
	0x10a: "senvcfg", // Supervisor environment configuration register.

	// Supervisor Counter Setup.
	0x120: "scountinhibit", // Supervisor counter-inhibit register.

	// Supervisor Trap Handling.
	0x140: "sscratch",  // Supervisor scratch register.
	0x141: "sepc",      // Supervisor exception program counter.
	0x142: "scause",    // Supervisor trap cause.
	0x143: "stval",     // Supervisor trap value.
	0x144: "sip",       // Supervisor interrupt pending.
	0xda0: "scountovf", // Supervisor count overflow.

	// Supervisor Indirect.
	0x150: "siselect", // Supervisor indirect register select.
	0x151: "sireg",    // Supervisor indirect register alias.
	0x152: "sireg2",   // Supervisor indirect register alias 2.
	0x153: "sireg3",   // Supervisor indirect register alias 3.
	0x155: "sireg4",   // Supervisor indirect register alias 4.
	0x156: "sireg5",   // Supervisor indirect register alias 5.
	0x157: "sireg6",   // Supervisor indirect register alias 6.

	// Supervisor Protection and Translation.
	0x180: "satp", // Supervisor address translation and protection.

	// Supervisor Timer Compare.
	0x14d: "stimecmp", // Supervisor timer compare.

	// Debug/Trace Registers.
	0x5a8: "scontext", // Supervisor-mode context register.

	// Supervisor Resource Management Configuration.
	0x181: "srmcfg", // Supervisor Resource Management Configuration.

	// Supervisor State Enable Registers.
	0x10c: "sstateen0", // Supervisor State Enable 0 Register.
	0x10d: "sstateen1", // Supervisor State Enable 1 Register.
	0x10e: "sstateen2", // Supervisor State Enable 2 Register.
	0x10f: "sstateen3", // Supervisor State Enable 3 Register.

	// Supervisor Control Transfer Records (SCTR) Configuration.
	0x14e: "sctrctl",    // SCTR Control Register.
	0x14f: "sctrstatus", // SCTR Status Register.
	0x15f: "sctrdepth",  // SCTR Depth Register.

	// Hypervisor and VS CSR addresses.

	// Hypervisor Trap Setup.
	0x600: "hstatus",    // Hypervisor status register.
	0x602: "hedeleg",    // Hypervisor exception delegation register.
	0x603: "hideleg",    // Hypervisor interrupt delegation register.
	0x604: "hie",        // Hypervisor interrupt-enable register.
	0x606: "hcounteren", // Hypervisor counter enable.
	0x607: "hgeie",      // Hypervisor guest external interrupt-enable register.

	// Hypervisor Trap Handling.
	0x643: "htval",  // Hypervisor trap value.
	0x644: "hip",    // Hypervisor interrupt pending.
	0x645: "hvip",   // Hypervisor virtual interrupt pending.
	0x64a: "htinst", // Hypervisor trap instruction (transformed).
	0xe12: "hgeip",  // Hypervisor guest external interrupt pending.

	// Hypervisor Configuration.
	0x60a: "henvcfg", // Hypervisor environment configuration register.

	// Hypervisor Protection and Translation.
	0x680: "hgatp", // Hypervisor guest address translation and protection.

	// Debug/Trace Registers.
	0x6a8: "hcontext", // Hypervisor-mode context register.

	// Hypervisor Counter/Timer Virtualization Registers.
	0x605: "htimedelta", // Delta for VS/VU-mode timer.

	// Hypervisor State Enable Registers.
	0x60c: "hstateen0", // Hypervisor State Enable 0 Register.
	0x60d: "hstateen1", // Hypervisor State Enable 1 Register.
	0x60e: "hstateen2", // Hypervisor State Enable 2 Register.
	0x60f: "hstateen3", // Hypervisor State Enable 3 Register.

	// Virtual Supervisor Registers.
	0x200: "vsstatus",  // Virtual supervisor status register.
	0x204: "vsie",      // Virtual supervisor interrupt-enable register.
	0x205: "vstvec",    // Virtual supervisor trap handler base address.
	0x240: "vsscratch", // Virtual supervisor scratch register.
	0x241: "vsepc",     // Virtual supervisor exception program counter.
	0x242: "vscause",   // Virtual supervisor trap cause.
	0x243: "vstval",    // Virtual supervisor trap value.
	0x244: "vsip",      // Virtual supervisor interrupt pending.
	0x280: "vsatp",     // Virtual supervisor address translation and protection

	// Virtual Supervisor Indirect.
	0x250: "vsiselect", // Virtual supervisor indirect register select.
	0x251: "vsireg",    // Virtual supervisor indirect register alias.
	0x252: "vsireg2",   // Virtual supervisor indirect register alias 2.
	0x253: "vsireg3",   // Virtual supervisor indirect register alias 3.
	0x255: "vsireg4",   // Virtual supervisor indirect register alias 4.
	0x256: "vsireg5",   // Virtual supervisor indirect register alias 5.
	0x257: "vsireg6",   // Virtual supervisor indirect register alias 6.

	// Virtual Supervisor Timer Compare.
	0x24d: "vstimecmp", // Virtual supervisor timer compare.

	// Virtual Supervisor Control Transfer Records (VSCTR) Configuration.
	0x24e: "vsctrctl", // VSCTR Control Register.

	// Machine-level CSR addresses.

	// Machine Information Registers.
	0xf11: "mvendorid",  // Vendor ID.
	0xf12: "marchid",    // Architecture ID.
	0xf13: "mimpid",     // Implementation ID.
	0xf14: "mhartid",    // Hardware thread ID.
	0xf15: "mconfigptr", // Pointer to configuration data structure.

	// Machine Trap Setup.
	0x300: "mstatus",    // Machine status register.
	0x301: "misa",       // ISA and extensions.
	0x302: "medeleg",    // Machine exception delegation register.
	0x303: "mideleg",    // Machine interrupt delegation register.
	0x304: "mie",        // Machine interrupt-enable register.
	0x305: "mtvec",      // Machine trap-handler base address.
	0x306: "mcounteren", // Machine counter enable.

	// Machine Trap Handling.
	0x340: "mscratch", // Machine scratch register.
	0x341: "mepc",     // Machine exception program counter.
	0x342: "mcause",   // Machine trap cause.
	0x343: "mtval",    // Machine trap value.
	0x344: "mip",      // Machine interrupt pending.
	0x34a: "mtinst",   // Machine trap instruction (transformed).
	0x34b: "mtval2",   // Machine second trap value.

	// Machine Indirect.
	0x350: "miselect", // Machine indirect register select.
	0x351: "mireg",    // Machine indirect register alias.
	0x352: "mireg2",   // Machine indirect register alias 2.
	0x353: "mireg3",   // Machine indirect register alias 3.
	0x355: "mireg4",   // Machine indirect register alias 4.
	0x356: "mireg5",   // Machine indirect register alias 5.
	0x357: "mireg6",   // Machine indirect register alias 6.

	// Machine Configuration.
	0x30a: "menvcfg", // Machine environment configuration register.
	0x747: "mseccfg", // Machine security configuration register.

	// Machine Memory Protection.
	0x3a0: "pmpcfg0", // Physical memory protection configuration.
	0x3a2: "pmpcfg2",
	0x3a4: "pmpcfg4",
	0x3a6: "pmpcfg6",
	0x3a8: "pmpcfg8",
	0x3aa: "pmpcfg10",
	0x3ac: "pmpcfg12",
	0x3ae: "pmpcfg14",
	0x3b0: "pmpaddr0", // Physical memory protection address register.
	0x3b1: "pmpaddr1",
	0x3b2: "pmpaddr2",
	0x3b3: "pmpaddr3",
	0x3b4: "pmpaddr4",
	0x3b5: "pmpaddr5",
	0x3b6: "pmpaddr6",
	0x3b7: "pmpaddr7",
	0x3b8: "pmpaddr8",
	0x3b9: "pmpaddr9",
	0x3ba: "pmpaddr10",
	0x3bb: "pmpaddr11",
	0x3bc: "pmpaddr12",
	0x3bd: "pmpaddr13",
	0x3be: "pmpaddr14",
	0x3bf: "pmpaddr15",
	0x3c0: "pmpaddr16",
	0x3c1: "pmpaddr17",
	0x3c2: "pmpaddr18",
	0x3c3: "pmpaddr19",
	0x3c4: "pmpaddr20",
	0x3c5: "pmpaddr21",
	0x3c6: "pmpaddr22",
	0x3c7: "pmpaddr23",
	0x3c8: "pmpaddr24",
	0x3c9: "pmpaddr25",
	0x3ca: "pmpaddr26",
	0x3cb: "pmpaddr27",
	0x3cc: "pmpaddr28",
	0x3cd: "pmpaddr29",
	0x3ce: "pmpaddr30",
	0x3cf: "pmpaddr31",
	0x3d0: "pmpaddr32",
	0x3d1: "pmpaddr33",
	0x3d2: "pmpaddr34",
	0x3d3: "pmpaddr35",
	0x3d4: "pmpaddr36",
	0x3d5: "pmpaddr37",
	0x3d6: "pmpaddr38",
	0x3d7: "pmpaddr39",
	0x3d8: "pmpaddr40",
	0x3d9: "pmpaddr41",
	0x3da: "pmpaddr42",
	0x3db: "pmpaddr43",
	0x3dc: "pmpaddr44",
	0x3dd: "pmpaddr45",
	0x3de: "pmpaddr46",
	0x3df: "pmpaddr47",
	0x3e0: "pmpaddr48",
	0x3e1: "pmpaddr49",
	0x3e2: "pmpaddr50",
	0x3e3: "pmpaddr51",
	0x3e4: "pmpaddr52",
	0x3e5: "pmpaddr53",
	0x3e6: "pmpaddr54",
	0x3e7: "pmpaddr55",
	0x3e8: "pmpaddr56",
	0x3e9: "pmpaddr57",
	0x3ea: "pmpaddr58",
	0x3eb: "pmpaddr59",
	0x3ec: "pmpaddr60",
	0x3ed: "pmpaddr61",
	0x3ee: "pmpaddr62",
	0x3ef: "pmpaddr63",

	// Machine State Enable Registers.
	0x30c: "mstateen0",
	0x30d: "mstateen1",
	0x30e: "mstateen2",
	0x30f: "mstateen3",

	// Machine Non-Maskable Interrupt Handling.
	0x740: "mnscratch", // Resumable NMI scratch register.
	0x741: "mnepc",     // Resumable NMI program counter.
	0x742: "mncause",   // Resumable NMI cause.
	0x744: "mnstatus",  // Resumable NMI status.

	// Machine Counter/Timers.
	0xb00: "mcycle",       // Machine cycle counter.
	0xb02: "minstret",     // Machine instructions-retired counter.
	0xb03: "mhpmcounter3", // Machine performance-monitoring counter.
	0xb04: "mhpmcounter4",
	0xb05: "mhpmcounter5",
	0xb06: "mhpmcounter6",
	0xb07: "mhpmcounter7",
	0xb08: "mhpmcounter8",
	0xb09: "mhpmcounter9",
	0xb0a: "mhpmcounter10",
	0xb0b: "mhpmcounter11",
	0xb0c: "mhpmcounter12",
	0xb0d: "mhpmcounter13",
	0xb0e: "mhpmcounter14",
	0xb0f: "mhpmcounter15",
	0xb10: "mhpmcounter16",
	0xb11: "mhpmcounter17",
	0xb12: "mhpmcounter18",
	0xb13: "mhpmcounter19",
	0xb14: "mhpmcounter20",
	0xb15: "mhpmcounter21",
	0xb16: "mhpmcounter22",
	0xb17: "mhpmcounter23",
	0xb18: "mhpmcounter24",
	0xb19: "mhpmcounter25",
	0xb1a: "mhpmcounter26",
	0xb1b: "mhpmcounter27",
	0xb1c: "mhpmcounter28",
	0xb1d: "mhpmcounter29",
	0xb1e: "mhpmcounter30",
	0xb1f: "mhpmcounter31",

	// Machine Counter Setup.
	0x320: "mcountinhibit", // Machine counter-inhibit register.
	0x321: "mcyclecfg",     // Machine cycle counter configuration register.
	0x322: "minstretcfg",   // Machine instret counter configuration register.
	0x323: "mhpmevent3",    // Machine performance-monitoring event selector.
	0x324: "mhpmevent4",
	0x325: "mhpmevent5",
	0x326: "mhpmevent6",
	0x327: "mhpmevent7",
	0x328: "mhpmevent8",
	0x329: "mhpmevent9",
	0x32a: "mhpmevent10",
	0x32b: "mhpmevent11",
	0x32c: "mhpmevent12",
	0x32d: "mhpmevent13",
	0x32e: "mhpmevent14",
	0x32f: "mhpmevent15",
	0x330: "mhpmevent16",
	0x331: "mhpmevent17",
	0x332: "mhpmevent18",
	0x333: "mhpmevent19",
	0x334: "mhpmevent20",
	0x335: "mhpmevent21",
	0x336: "mhpmevent22",
	0x337: "mhpmevent23",
	0x338: "mhpmevent24",
	0x339: "mhpmevent25",
	0x33a: "mhpmevent26",
	0x33b: "mhpmevent27",
	0x33c: "mhpmevent28",
	0x33d: "mhpmevent29",
	0x33e: "mhpmevent30",
	0x33f: "mhpmevent31",

	// Machine Control Transfer Records Configuration.
	0x34e: "mctrctl", // Machine Control Transfer Records Control Register.

	// Debug/Trace Registers (shared with Debug Mode).
	0x7a0: "tselect",  // Debug/Trace trigger register select.
	0x7a1: "tdata1",   // First Debug/Trace trigger data register.
	0x7a2: "tdata2",   // Second Debug/Trace trigger data register.
	0x7a3: "tdata3",   // Third Debug/Trace trigger data register.
	0x7a8: "mcontext", // Machine-mode context register.

	// Debug Mode Registers.
	0x7b0: "dcsr",      // Debug control and status register.
	0x7b1: "dpc",       // Debug program counter.
	0x7b2: "dscratch0", // Debug scratch register 0.
	0x7b3: "dscratch1", // Debug scratch register 1.
}

func csrName(csr int) string {
	name, ok := csrs[csr]
	if ok {
		return fmt.Sprintf("%s[%03x]", name, csr)
	}
	return fmt.Sprintf("%03x", csr)
}

func (cpu *CPU) GetCSR(csr CSR) (uint64, error) {
	if cpu.Mode() < csr.Privilege() && false {
		return 0, cpu.Trap(isa.CauseIllegalInstr, uint64(csr),
			fmt.Errorf("GetCSR(%x), mode=%v", csr, cpu.Mode()))
	}
	// Handle read-only CSRs here by returning the fixed or computed
	// value.

	var v uint64

	switch csr {
	case CsrMstatus:
		v = uint64(cpu.mstatus)

	case CsrMisa:
		v = cpu.CSR[csr]
		v |= isa.MisaMXL |
			isa.MisaA | // A (Atomic)
			isa.MisaC | // C (Compressed)
			isa.MisaD | // D (Double)
			isa.MisaF | // F (Float)
			isa.MisaG | // G (Additional alias for IMAFD)
			isa.MisaI | // I (Integer)
			isa.MisaM | // M (Multiply)
			isa.MisaS | // S (Supervisor)
			isa.MisaU // U (User mode)

		// Debug triggers.
	case 0x7a0, 0x7a1, 0x7a2, 0x7a3, 0x7a4:

	case CsrCycle:
		v = cpu.Time

	case CsrTime:
		v = cpu.syncTime()

	case CsrInstret:
		v = cpu.Instret

	case CsrMvendorid:

	case CsrMarchid:
		v = 0x100

	case CsrMimpid:
		v = 0x1

	case CsrMhartid:

	case CsrScountinhibit:

	case CsrSstatus:
		v = uint64(cpu.mstatus & isa.SstatusMask)

	case CsrSie:
		mask := uint64(isa.IntSSIP | isa.IntSTIP | isa.IntSEIP)
		v = cpu.CSR[CsrMie] & mask

	case CsrSip:
		mask := uint64(isa.IntSSIP | isa.IntSTIP | isa.IntSEIP)
		v = cpu.CSR[CsrMip] & mask

		// Vector extension.

	case CsrVstart:
		v = cpu.vpu.VStart

	case CsrVl:
		v = cpu.vpu.VL

	case CsrVtype:
		v = uint64(cpu.vpu.VType)

	case CsrVlenb:
		v = uint64(cpu.vpu.VLEN / 8)

	case CsrSeed:
		var buf [2]byte
		_, err := rand.Read(buf[:])
		if err != nil {
			return 0, err
		}
		v = uint64(0x80000000)
		v |= uint64(buf[0]) << 8
		v |= uint64(buf[1])

	case CsrGoemuTime:
		v = uint64(time.Now().UnixNano())

	default:
		if csr >= 0xb03 && csr <= 0xb1f {
			// Mhpmcounters
		} else {
			v = cpu.CSR[csr]
		}
	}

	if debugCSR {
		log.Printf("GetCSR(%v): %v", csr, v)
	}

	return v, nil
}

func (cpu *CPU) SetCSR(csr CSR, v uint64) error {
	return cpu.SetCSRX(csr, v, 0, isa.Instr{})
}

func (cpu *CPU) SetCSRX(csr CSR, v uint64, raw uint32, instr isa.Instr) error {

	if debugCSR {
		log.Printf("SetCSR(%v, %v)", csr, v)
	}

	if cpu.Mode() < csr.Privilege() && false {
		return cpu.Trap(isa.CauseIllegalInstr, uint64(csr),
			fmt.Errorf("SetCSR(%x)=%v, mode=%v", csr, v, cpu.Mode()))
	}
	if csr.ReadOnly() {
		return cpu.Trap(isa.CauseIllegalInstr, uint64(csr),
			fmt.Errorf("SetCSR(%x)=%v: read-only", csr, v))
	}

	// Handle read-only and functional CSRs here by ignoring update or
	// by updating CPU state accordingly.
	switch csr {
	case CsrMstatus:
		cpu.mstatus = isa.Mstatus(v)

	case CsrMisa:
		cpu.CSR[csr] = v

	case CsrMie, CsrMip:
		cpu.CSR[csr] = v

	case CsrSstatus:
		cpu.mstatus = (cpu.mstatus & ^isa.SstatusMask) |
			(isa.Mstatus(v) & isa.SstatusMask)

	case CsrStimecmp:
		cpu.CSR[csr] = v
		if cpu.syncTime() < v {
			// Next timer interrupt in the future, clear interrupts.
			cpu.CSR[CsrMip] &^= isa.IntSTIP
		}

	case CsrSie:
		// sie is a masked view of mie — only S-mode bits
		mask := uint64(isa.IntSSIP | isa.IntSTIP | isa.IntSEIP)
		mie := cpu.CSR[CsrMie]
		cpu.CSR[CsrMie] = (mie & ^mask) | (v & mask)

	case CsrSip:
		// sip is a masked view of mip — only S-mode bits.
		mask := uint64(isa.IntSSIP | isa.IntSTIP | isa.IntSEIP)
		mip := cpu.CSR[CsrMip]
		cpu.CSR[CsrMip] = (mip & ^mask) | (v & mask)

	case CsrSatp:
		satp := mmu.Satp(v)
		cpu.MMU.SetSatp(satp)
		cpu.codePagenum = 0
		cpu.codePage = nil

		// Save Satp to CSR so that it can be queried.
		cpu.CSR[csr] = v

		if cpu.Trace {
			cpu.traceFunc(cpu.PC)
			cpu.tracef(raw, instr, "Satp: %v", satp)
		}

	case CsrGoemuDebug:
		cpu.DebugTrace = v&0b1 != 0
		cpu.CSR[csr] = v

	case CsrGoemuCPUProfile:
		if v == 0 {
			cpu.csr7c2Refcount--
			if cpu.csr7c2Refcount <= 0 {
				pprof.StopCPUProfile()
				cpu.csr7c2File.Sync()
			}
		} else {
			if cpu.csr7c2Refcount == 0 {
				var err error
				if cpu.csr7c2File == nil {
					cpu.csr7c2File, err = os.Create(cpu.CSR7c2Filename)
					if err != nil {
						return err
					}
				}
				err = pprof.StartCPUProfile(cpu.csr7c2File)
				if err != nil {
					return err
				}
			}
			cpu.csr7c2Refcount++
		}

	default:
		cpu.CSR[csr] = v
	}

	return nil
}

func (cpu *CPU) Mstatus() isa.Mstatus {
	return cpu.mstatus
}

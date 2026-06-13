# RISC-V Architecture and Concepts

## The Boot Sequence

1. **Prepare the DTB (Device Tree Blob):** This is a small data
   structure that tells Linux "The UART is at 0x10000000 and I have
   512MB of RAM."
2. **Load the Image:** Put the Linux `Image` binary at `0x80200000`.
3. **Set Initial Registers:**
   * `a0 = 0` (The Hart ID)
   * `a1 = 0x82000000` (Address of the DTB)
   * `PC = 0x80200000`

## Supervisor Model

### Mode Transition

In RISC-V, mode transitions are governed by privileged instructions
(`ecall`, `ebreak`, `sret`, `mret`) and hardware interrupts. When an
exception or interrupt occurs, the processor transitions to a higher
(or equal) privilege mode to handle it. Conversely, returning from a
handler restores the previous state.

Here is the foundational breakdown of how these transitions work,
filling out the table for **User (U)**, **Supervisor (S)**, and
**Machine (M)** modes.

#### Key Concepts & CSR Fields

Before looking at the table, it helps to understand what the actions
mean:

* **`xPP` (Previous Privilege):** Holds the mode the processor was in
  *before* the trap occurred ($U=00$, $S=01$, $M=11$).
* **`xPIE` (Previous Interrupt Enable):** Saves the state of the
  interrupt enable bit (`xIE`) from before the trap.
* **`xIE` (Interrupt Enable):** Gets set to `0` during a trap to
  globally disable interrupts while entering the handler.
* **`xtvec` / `xepc`:** The target address is determined by the trap
  vector register (`mtvec`/`stvec`), and the return address is saved
  in the exception program counter (`mepc`/`sepc`).

### RISC-V State Transition Table

By default, traps go to **M-mode** unless they are explicitly
delegated to **S-mode** using delegation registers (`medeleg` for
exceptions, `mideleg` for interrupts).

| Input  | Mode | Cond   | Cause  | Actions                             | Tgt |
|--------|------|--------|--------|-------------------------------------|-----|
| ecall  | M    | —      | EcallM | MPP=M, MPIE=MIE, MIE=0, mepc=pc     | M   |
| ecall  | S    | !Deleg | EcallS | MPP=S, MPIE=MIE, MIE=0, mepc=pc     | M   |
| ecall  | S    | Deleg  | EcallS | SPP=S, SPIE=SIE, SIE=0, sepc=pc     | S   |
| ecall  | U    | !Deleg | EcallU | MPP=U, MPIE=MIE, MIE=0, mepc=pc     | M   |
| ecall  | U    | Deleg  | EcallU | SPP=U, SPIE=SIE, SIE=0, sepc=pc     | S   |
| ebreak | Any  | !Deleg | Break  | MPP=mode, MPIE=MIE, MIE=0, mepc=pc  | M   |
| ebreak | Any  | Deleg  | Break  | SPP=mode, SPIE=SIE, SIE=0, sepc=pc  | S   |
| mret   | M    | —      | —      | MIE=MPIE, MPIE=1, mode=MPP, pc=mepc | MPP |
| sret   | S    | —      | —      | SIE=SPIE, SPIE=1, mode=SPP, pc=sepc | SPP |
| Intr   | Any  | !Deleg | Intr   | MPP=mode, MPIE=MIE, MIE=0, mepc=pc  | M   |
| Intr   | Any  | Deleg  | Intr   | SPP=mode, SPIE=SIE, SIE=0, sepc=pc  | S   |


### Key Takeaways from the Table

1. **The Delegation Rule:** Notice how for `U` and `S` modes, the
   outcome depends on whether the trap is delegated. If
   `medeleg[cause]` or `mideleg[cause]` is set to 1, the trap bypasses
   M-mode entirely and updates the **S-mode** CSRs (`sstatus`, `sepc`,
   `stvec`).

2. **The Return Mechanism (`xret`):** When executing `mret`, the
   hardware reads the `MPP` field to know what mode to drop back
   into. It also restores the interrupt state (`MIE = MPIE`) and
   resets `MPIE` to 1.

3. **Interrupts vs. Exceptions:** An `ecall` or `ebreak` is
   synchronous (the saved `epc` points to the instruction itself). An
   interrupt is asynchronous, meaning `epc` points to the next
   instruction that *would* have been executed.

4. **M-Mode Interrupts:** If an interrupt occurs while already running
   in M-mode, it cannot be delegated to S-mode (global RISC-V rule:
   traps can never be delegated to a lower privilege mode than the one
   they occurred in). The table still holds true because Deleg
   implicitly evaluates to false if Mode == M.

### M and S mode trap CSR blocks

The full M-mode trap CSR block is:

| CSR        | Address | Purpose                                              |
|------------|---------|------------------------------------------------------|
| `mstatus`  | `0x300` | Global status (MPP, MIE, MPIE, etc.)                 |
| `mtvec`    | `0x305` | Trap vector base address                             |
| `mscratch` | `0x340` | Scratch register for M-mode handler                  |
| `mepc`     | `0x341` | PC of trapping instruction                           |
| `mcause`   | `0x342` | Exception/interrupt cause code                       |
| `mtval`    | `0x343` | Trap value (faulting address, bad instruction, etc.) |
| `mip`      | `0x344` | Machine interrupt pending                            |

And the S-mode equivalents (for completeness):

| CSR        | Address | Purpose                             |
|------------|---------|-------------------------------------|
| `sstatus`  | `0x100` | Subset of mstatus visible to S-mode |
| `stvec`    | `0x105` | S-mode trap vector                  |
| `sscratch` | `0x140` | Scratch for S-mode handler          |
| `sepc`     | `0x141` | S-mode exception PC                 |
| `scause`   | `0x142` | S-mode cause                        |
| `stval`    | `0x143` | S-mode trap value                   |
| `sip`      | `0x144` | S-mode interrupt pending            |

### MMU

| Mode | PTE (U, R, W, X)   | SUM      | MXR      | Read    | Store   | Exec |
| ---- | :----------------- | :------- | :------- | :------ | :------ | :--- |
| U    | U=0 (Any R/W/X)    | —        | —        | No      | No      | No   |
| U    | U=1, R=1, W=1, X=0 | —        | —        | yes     | yes     | No   |
| U    | U=1, R=0, W=0, X=1 | —        | MXR=0    | No      | No      | yes  |
| U    | U=1, R=0, W=0, X=1 | —        | MXR=1    | yes     | No      | yes  |
| S    | U=1 (Any R/W/X)    | SUM=0    | —        | No      | No      | No   |
| S    | U=1, R=1, W=1, X=0 | SUM=1    | —        | yes     | yes     | No   |
| S    | U=1, R=0, W=0, X=1 | SUM=1    | MXR=0    | No      | No      | No¹  |
| S    | U=0, R=1, W=0, X=1 | —        | MXR=0    | yes     | No      | yes  |
| S    | U=0, R=0, W=0, X=1 | —        | MXR=1    | yes     | No      | yes  |
| M    | (Any Page)         | —        | —        | yes²    | yes²    | yes² |
| M    | (Any Page)         | mstatus³ | mstatus³ | Match S | Match S | No   |

**Architectural Rules & Pitfalls**

1. The S-Mode Execution Trap: Notice row 7. Even if SUM=1, an attempt
   by Supervisor mode to fetch an instruction from a User page (U=1)
   will always trigger an instruction page fault. This is a hardcoded
   security feature to prevent "ret2usr" exploits where a kernel is
   tricked into running malicious user-space code.

2. M-Mode and the MPRV Exception: Normally, if the Modify Privilege
   bit is unset (mstatus.MPRV=0), Machine mode bypasses the MMU
   entirely.

2. However, if the Modify Privilege bit is set (mstatus.MPRV=1), the
   MMU steps in only for data loads and stores (not fetches). It
   translates those accesses using the privilege level specified in
   mstatus.MPP (usually S or U mode), meaning SUM and MXR suddenly
   apply to M-mode data operations too. The `Read` and `Store`
   operations match S-mode semantics.


## Device Tree Blob (DTB)

The canonical documentation for the Device Tree Blob (DTB) format is
the **Devicetree Specification**, currently maintained by
[Devicetree.org](https://www.devicetree.org). Specifically, **Chapter
5: Flattened Devicetree (DTB) Format** contains the exact memory
layout.

The DTB (also known as a Flattened Devicetree or FDT) is a linear,
pointerless data structure. When loaded into memory, it must follow a
specific sequence of blocks.

### 1. High-Level Memory Layout

A DTB file consists of four main sections, which must appear in the
following order:

1. **fdt_header**: A fixed-size header containing magic numbers and offsets.
2. **memory reservation block**: A list of memory areas the kernel must not use.
3. **structure block**: The actual tree (nodes and properties) encoded as a series of tokens.
4. **strings block**: All property names are stored here as null-terminated strings to save space.

---

### 2. The Header (`fdt_header`)

The header is the "entry point" for any parser. **Note:** All fields
are 32-bit integers stored in **Big-Endian** format.

| Field           | Offset | Description                                    |
| ---             | ---    | ---                                            |
| magic           | 0x00   | The constant 0xd00dfeed                        |
| totalsize       | 0x04   | Total size of the DTB in bytes                 |
| off_dt_struct   | 0x08   | Offset from header to Structure Block          |
| off_dt_strings  | 0x0C   | Offset from header to Strings Block.           |
| off_mem_rsvmap  | 0x10   | Offset from header to Memory Reservation Block |
| version         | 0x14   | Format version (standard is `17`)              |
| last_comp_ver   | 0x18   | Last compatible version (usually `16`)         |
| boot_cpuid_phys | 0x1C   | Physical ID of the system's boot CPU           |
| size_dt_strings | 0x20   | Length of the Strings Block in bytes           |
| size_dt_struct  | 0x24   | Length of the Structure Block in bytes         |

---

### 3. Structure Block (The "Tree")

The tree is parsed as a stream of tokens. Each token is a 32-bit
Big-Endian integer:

* **`FDT_BEGIN_NODE` (0x00000001)**: Followed by the null-terminated name of the node (padded to 4-byte alignment).
* **`FDT_END_NODE` (0x00000002)**: No data follows.
* **`FDT_PROP` (0x00000003)**: Followed by:
1. `uint32 len`: Length of the property's value.
2. `uint32 nameoff`: Offset within the **Strings Block** for the property name.
3. `data`: The actual value bytes (padded to 4-byte alignment).


* **`FDT_NOP` (0x00000004)**: Ignored.
* **`FDT_END` (0x00000009)**: Ends the entire structure block.

### 4. Implementation Notes

The emulator is targeting RISC-V, note:

1. **Endianness**: emulator's internal memory is Little-Endian (RISC-V
   standard), but the DTB is **strictly Big-Endian**. Use
   `binary.BigEndian` to read the header and tokens.
2. **Alignment**: Every section and property value must be aligned to
   a **4-byte boundary**. If a node name is "uart", it takes 4 bytes +
   1 null terminator = 5 bytes; you must skip 3 padding bytes before
   the next token.
3. **Passing to Linux**: The RISC-V ABI requires qthe **physical
   address** of the DTB header to be stored into register **`a1`**.

> **Canonical URL**: You can find the latest stable version (v0.4) of the full specification here: [https://www.devicetree.org/specifications/](https://www.devicetree.org/specifications/)

## [OASIS VirtIO (Virtual I/O)](https://docs.oasis-open.org/virtio/virtio/v1.3/virtio-v1.3.html)

| Device                        | Linux Driver   | Device ID |
| :-----                        | :----          | -----:    |
| Block Device                  | virtio_blk     | 2         |
| 9P Transport                  | 9pnet_virtio   | 9         |
| Network Card                  | virtio_net     | 1         |
| Entropy Source / RNG          | virtio_rng     | 4         |
| Cryptographic Accelerator     | virtio_crypto  | 20        |
| Persistent Error Storage      | virtio_pstore  | 22        |
| Graphics Adapter / GPU        | virtio_gpu     | 16        |
| Input Subsystem               | virtio_input   | 18        |
| Console / Multi-Stream Serial | virtio_console | 3         |


## RISC-V Timer Interrupt Generation and Handling

### Overview

This diagram shows the complete lifecycle of a RISC-V timer interrupt
using the SSTC (Supervisor-mode Timer Compare) extension.

### Key Components

- **CPU (Hart)**: The RISC-V processor core
- **Hardware Timer**: Counter that always increments (`CSR_TIME`)
- **Timer Compare Unit**: Compares `CSR_TIME` against `CSR_STIMECMP`
- **Interrupt Controller**: Manages the timer interrupt pending bit (`MIP.STIP`)
- **Linux Kernel**: Handles the timer interrupt and schedules next events

### Complete Interrupt Lifecycle

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        PHASE 1: INITIALIZATION                              │
└─────────────────────────────────────────────────────────────────────────────┘

Time: t0
────────────────────────────────────────────────────────────────────────────────

[1] Kernel calls: riscv_timer_starting_cpu() [line 108]
    │
    ├─> riscv_clock_event_stop() [line 113]
    │   └─> CSR_STIMECMP = ULONG_MAX (0xffffffffffffffff)
    │       │
    │       └─ Hardware Timer Logic:
    │          CSR_TIME (current) < ULONG_MAX
    │          └─> MIP.STIP = 0 (no interrupt)
    │
    └─> clockevents_config_and_register() [line 121]
        └─> Prepares timer device for use


┌─────────────────────────────────────────────────────────────────────────────┐
│               PHASE 2: SCHEDULING FIRST TIMER EVENT                         │
└─────────────────────────────────────────────────────────────────────────────┘

Time: t1 (when userspace calls nanosleep or similar)
────────────────────────────────────────────────────────────────────────────────

[2] Kernel calculates next event time:
    │
    ├─> clockevent framework decides:
    │   next_event_time = current_time + delta_ns
    │
    └─> Calls: riscv_clock_next_event(delta) [line 46]
        │
        ├─> Line 49: next_tval = get_cycles64() + delta
        │            (Read CSR_TIME, add delta to get absolute time)
        │
        └─> Line 57: csr_write(CSR_STIMECMP, next_tval)
            │
            │   ╔════════════════════════════════════════════════════════════╗
            │   ║  HARDWARE TIMER COMPARISON LOGIC (Always Running)          ║
            │   ╠════════════════════════════════════════════════════════════╣
            │   ║                                                            ║
            │   ║  Condition Check (runs every cycle):                       ║
            │   ║  ┌──────────────────────────────────────────────────────┐  ║
            │   ║  │ if (CSR_TIME >= CSR_STIMECMP) then:                  │  ║
            │   ║  │     Set MIP.STIP = 1  (Interrupt Pending)            │  ║
            │   ║  │ else:                                                │  ║
            │   ║  │     Keep MIP.STIP = 0  (No interrupt)                │  ║
            │   ║  └──────────────────────────────────────────────────────┘  ║
            │   ║                                                            ║
            │   ║  This check happens combinatorially (no delay)             ║
            │   ║                                                            ║
            │   ╚════════════════════════════════════════════════════════════╝
            │
            └─> Return 0 (success)


HARDWARE STATE AT THIS POINT:
─────────────────────────────
CSR_TIME     = t1                    (current hardware counter value)
CSR_STIMECMP = t1 + delta            (compare register set)
MIP.STIP     = 0                     (no interrupt yet, since t1 < t1+delta)


┌─────────────────────────────────────────────────────────────────────────────┐
│              PHASE 3: WAITING FOR TIMER TO FIRE                             │
└─────────────────────────────────────────────────────────────────────────────┘

Time: t1 < t < (t1 + delta)
────────────────────────────────────────────────────────────────────────────────

[3] Hardware Timer Continuously Increments CSR_TIME:
    │
    ├─> CPU clock cycles → CSR_TIME increases
    │
    ├─> Each cycle:
    │   ├─> Hardware compares:
    │   │   if (CSR_TIME >= CSR_STIMECMP) then MIP.STIP = 1
    │   │
    │   └─> Current state:
    │       CSR_TIME = t1 + (t - t1)         [current time]
    │       CSR_STIMECMP = t1 + delta        [target time]
    │       Comparison: (t - t1) < delta?
    │       MIP.STIP = 0 (still waiting)
    │
    └─> CPU can execute user code normally
        (If CSR_SIE.STIE = 1, will take interrupt when MIP.STIP = 1)


TIMELINE OF CSR_TIME:
─────────────────────
t1              t1+delta/2           t1+delta
│               │                    │
├───────────────┼────────────────────┤
Pending         Waiting              INTERRUPT FIRES HERE
CSR_TIME < STIMECMP
MIP.STIP = 0


┌─────────────────────────────────────────────────────────────────────────────┐
│           PHASE 4: TIMER FIRES - INTERRUPT IS GENERATED                     │
└─────────────────────────────────────────────────────────────────────────────┘

Time: t1 + delta (exact moment)
────────────────────────────────────────────────────────────────────────────────

[4] Hardware Detects Compare Match:
    │
    ├─> Condition becomes true:
    │   CSR_TIME >= CSR_STIMECMP
    │   │
    │   └─> Hardware Action:
    │       Set MIP.STIP = 1
    │
    └─> This is a combinatorial result (no delay)


HARDWARE STATE:
───────────────
CSR_TIME     = t1 + delta
CSR_STIMECMP = t1 + delta
MIP.STIP     = 1                     ← INTERRUPT PENDING


┌─────────────────────────────────────────────────────────────────────────────┐
│       PHASE 5: INTERRUPT DELIVERY (With Conditions)                         │
└─────────────────────────────────────────────────────────────────────────────┘

Time: t1 + delta + ε (where ε is interrupt delivery latency)
────────────────────────────────────────────────────────────────────────────────

[5] Interrupt Controller Checks Enable Bits:
    │
    └─> Prerequisites for interrupt to be taken:
        │
        ├─ MIP.STIP = 1              ✓ (set by hardware)
        ├─ SIE.STIE = 1              ? (Software must set this)
        └─ Interrupts enabled (SIE bit in SSTATUS) = 1 ? (Software must set)
        │
        └─> If ALL conditions met:
            Interrupt is taken (CPU enters trap handler)


[6] Interrupt Trap Handler Execution:
    │
    ├─> CPU jumps to exception handler (via STVEC)
    │
    ├─> Low-level assembly handler is called
    │   (Saves context, calls high-level handler)
    │
    └─> high-level handler: riscv_timer_interrupt() [line 148]


CRITICAL POINT:
──────────────
At this point, the interrupt PENDING bit (MIP.STIP) is STILL SET!
It will remain set until software clears it.

MIP.STIP = 1 (remains set)


┌─────────────────────────────────────────────────────────────────────────────┐
│           PHASE 6: INTERRUPT HANDLER EXECUTION (Line 148-156)               │
└─────────────────────────────────────────────────────────────────────────────┘

Time: t1 + delta + ε + handler_latency
────────────────────────────────────────────────────────────────────────────────

[7] Line 150: evdev = this_cpu_ptr(&riscv_clock_event)
    │           [Get per-CPU clock event device]
    │
    └─> evdev points to the clock event device structure


[8] Line 152: riscv_clock_event_stop()
    │
    ├─> Executes: csr_write(CSR_STIMECMP, ULONG_MAX)
    │   │
    │   └─> Hardware Effect:
    │       CSR_STIMECMP = 0xffffffffffffffff
    │       │
    │       ├─> Comparison immediately runs:
    │       │   CSR_TIME (= t1 + delta) >= ULONG_MAX?
    │       │   NO! → MIP.STIP is CLEARED by hardware
    │       │
    │       └─ MIP.STIP = 0 (interrupt pending bit cleared)
    │
    └─> IMPORTANT: MIP.STIP is cleared NOT by explicit write,
        but by the hardware detecting that the compare condition
        is no longer true!


HARDWARE STATE AFTER riscv_clock_event_stop():
────────────────────────────────────────────────
CSR_TIME     = t1 + delta
CSR_STIMECMP = 0xffffffffffffffff (ULONG_MAX)
MIP.STIP     = 0 (hardware cleared it because CSR_TIME < ULONG_MAX)
              ↑ KEY POINT


[9] Line 153: evdev->event_handler(evdev)
    │
    ├─> This is a callback to: handle_oneshot_event() or similar
    │
    ├─> Which calls the clockevent framework:
    │   clockevents_program_event()
    │   │
    │   └─> This determines the NEXT timer event
    │       (Based on current system time and pending timers)
    │       │
    │       └─> Calls: riscv_clock_next_event(delta_new) [Line 46]
    │           │
    │           └─> Sets CSR_STIMECMP = get_cycles64() + delta_new
    │               │
    │               └─ Hardware Check:
    │                  Is CSR_TIME >= new_STIMECMP?
    │                  Usually NO (future time)
    │                  └─> MIP.STIP stays 0
    │
    └─> Function returns


NEW HARDWARE STATE (if delta_new is scheduled):
──────────────────────────────────────────────
CSR_TIME         = t1 + delta + ε
CSR_STIMECMP     = (t1 + delta + ε) + delta_new  [next event]
MIP.STIP         = 0
                   ↑ Ready for next interrupt


[10] Line 155: return IRQ_HANDLED
     │
     └─> Interrupt handler returns to kernel


┌─────────────────────────────────────────────────────────────────────────────┐
│         PHASE 7: EXCEPTION HANDLER CLEANUP & RETURN                         │
└─────────────────────────────────────────────────────────────────────────────┘

Time: t1 + delta + ε + handler_latency + cleanup
────────────────────────────────────────────────────────────────────────────────

[11] Low-level exception handler (assembly):
     │
     ├─> Restores user context
     │
     ├─> Executes SRET (Supervisor Exception Return)
     │
     ├─ CPU returns to execution BEFORE the interrupt
     │  (User program resumes from where it was blocked)
     │
     └─> User code continues (no longer blocked in nanosleep)


┌─────────────────────────────────────────────────────────────────────────────┐
│  PHASE 8: LOOP CONTINUES - NEXT TIMER EVENT WAITING                         │
└─────────────────────────────────────────────────────────────────────────────┘

Time: t1 + delta + ε to (t1 + delta + ε) + delta_new
────────────────────────────────────────────────────────────────────────────────

[12] Hardware Timer Continues Counting:
     │
     └─> Same process repeats from PHASE 3:
         CSR_TIME increments
         When CSR_TIME >= CSR_STIMECMP: MIP.STIP set to 1
         Interrupt fires → handler → next event scheduled


```

---

### Critical Hardware Behaviors

#### MIP.STIP (Machine/Supervisor Interrupt Pending) Bit Semantics

```
╔═════════════════════════════════════════════════════════════════════════════╗
║                    MIP.STIP Generation Logic (Hardware)                     ║
╠═════════════════════════════════════════════════════════════════════════════╣
║                                                                             ║
║  COMBINATORIAL LOGIC (No storage, pure comparison):                         ║
║  ┌─────────────────────────────────────────────────────────┐               ║
║  │  MIP.STIP = (CSR_TIME >= CSR_STIMECMP) ? 1 : 0          │               ║
║  └─────────────────────────────────────────────────────────┘               ║
║                                                                             ║
║  KEY POINTS:                                                                ║
║  ✓ NOT stored in a register                                                 ║
║  ✓ Computed in real-time based on current CSR values                        ║
║  ✓ No explicit "clear" instruction needed                                   ║
║  ✓ Cleared automatically when CSR_TIME < CSR_STIMECMP                       ║
║  ✓ Set automatically when CSR_TIME >= CSR_STIMECMP                          ║
║                                                                             ║
╚═════════════════════════════════════════════════════════════════════════════╝
```

#### What CLEARS MIP.STIP?

```
NOT this (no such instruction):
    csrc_clear(CSR_MIP, MIP_STIP)   // Does NOT work for timer bit!

BUT this happens automatically:
    csr_write(CSR_STIMECMP, ULONG_MAX)   // Now CSR_TIME < ULONG_MAX always
    └─> Hardware: CSR_TIME >= ULONG_MAX? NO!
        └─> MIP.STIP = 0 (automatically cleared)
```

#### What SETS MIP.STIP?

```
Automatically by hardware when:
    CSR_TIME crosses >= CSR_STIMECMP boundary

Example Timeline:

    Time:    CSR_TIME  CSR_STIMECMP  MIP.STIP
    ─────────────────────────────────────────
    t0       1000      5000          0
    t1       2000      5000          0
    t2       3000      5000          0
    t3       4999      5000          0
    t4       5000      5000          1  ← SET HERE (boundary crossed)
    t5       5001      5000          1  (still set)

    Handler writes: csr_write(CSR_STIMECMP, 10000)

    t6       5002      10000         0  ← CLEARED HERE (no longer true)
```

---

### The Bug Scenario: Why Timer Gets Stuck at 0xffffffffffffffff

#### Scenario: Emulator Implementation Bug

```
CORRECT HARDWARE BEHAVIOR:
═══════════════════════════

[1] Interrupt fires
    CSR_TIME ≥ CSR_STIMECMP = true
    MIP.STIP = 1 ✓

[2] Handler executes: csr_write(CSR_STIMECMP, ULONG_MAX)
    Hardware checks: CSR_TIME ≥ 0xffffffffffffffff? NO!
    MIP.STIP = 0 ✓ (cleared automatically)

[3] Handler calls riscv_clock_next_event(delta)
    CSR_STIMECMP = CSR_TIME + delta (future value)
    Hardware checks: CSR_TIME ≥ new_value? NO!
    MIP.STIP = 0 ✓ (stays 0)

═════════════════════════════════════════════════════════════════════════════

BUGGY EMULATOR BEHAVIOR:
════════════════════════

[1] Interrupt fires
    CSR_TIME ≥ CSR_STIMECMP = true
    MIP.STIP = 1 ✓

[2] Handler executes: csr_write(CSR_STIMECMP, ULONG_MAX)
    ┌─ Emulator BUG: CSR_STIMECMP write not properly affecting comparison
    │
    └─ Hardware check logic NOT executed
       OR comparison logic cached old value
       OR CSR write buffered/deferred

    MIP.STIP remains 1 ✗ (NOT cleared)

[3] Handler calls riscv_clock_next_event(delta)
    CSR_STIMECMP = CSR_TIME + delta
    ┌─ Emulator BUG: Write doesn't take effect
    │  OR: CSR_STIMECMP still showing 0xffffffffffffffff
    │  OR: Timer comparison not updated
    │
    └─ Next event never triggers
       Timer appears "stuck" at 0xffffffffffffffff

═════════════════════════════════════════════════════════════════════════════

RESULT:
═══════
Userspace sleep() never returns because:
  ✗ No interrupt delivered
  ✗ Timer left in "disabled" state
  ✗ Process blocked forever waiting for wakeup that never comes

```

---

### Emulator Implementation Checklist

For a software emulator to correctly implement RISC-V timer:

```
REQUIRED BEHAVIOR:
══════════════════

[ ] CSR_STIMECMP write immediately updates hardware state
    └─> Next hardware timer check uses new value

[ ] Hardware comparison (CSR_TIME >= CSR_STIMECMP) is
    combinatorial, not buffered or deferred

[ ] MIP.STIP is derived combinatorially:
    └─> MIP.STIP = (CSR_TIME >= CSR_STIMECMP)
    └─> Changes immediately when either CSR_TIME or
        CSR_STIMECMP changes

[ ] CSR_TIME increments every cycle (no skips)
    └─> Hardware must properly simulate time progression

[ ] When CSR_TIME crosses >= CSR_STIMECMP:
    └─> MIP.STIP immediately becomes 1
    └─> Interrupt is delivered to CPU

[ ] When CSR_STIMECMP is updated to beyond CSR_TIME:
    └─> CSR_TIME >= CSR_STIMECMP condition becomes false
    └─> MIP.STIP immediately becomes 0

[ ] CSR_TIME read/write operations are atomic

[ ] CSR_STIMECMP read/write operations are atomic

[ ] No race conditions between:
    ├─> CSR_STIMECMP write
    ├─> CSR_TIME increment
    └─> MIP.STIP generation
```

---

### What Could Go Wrong in Emulator

```
POTENTIAL BUGS:
═══════════════

1. CSR_STIMECMP Write Buffering
   ┌─────────────────────────────────────────────┐
   │ csr_write(CSR_STIMECMP, value)              │
   │                                             │
   │ Buffered write (WRONG):                     │
   │   queue_write(CSR_STIMECMP, value)          │
   │   [doesn't take effect until later]         │
   │                                             │
   │ Immediate write (CORRECT):                  │
   │   stimecmp_register = value                 │
   │   [immediately affects comparison]          │
   └─────────────────────────────────────────────┘

2. Cached Comparison Results
   ┌─────────────────────────────────────────────┐
   │ if (cached_comparison_result)               │
   │   mip_stip = 1                              │
   │                                             │
   │ WRONG: Caches comparison from last cycle    │
   │                                             │
   │ CORRECT: Recompute every cycle:             │
   │   mip_stip = (current_time >= stimecmp)     │
   └─────────────────────────────────────────────┘

3. Stale Interrupt Delivery
   ┌─────────────────────────────────────────────┐
   │ Hardware: MIP.STIP becomes 0                │
   │ But: CPU still delivers old pending irq     │
   │                                             │
   │ Result: Irq delivered with stale values     │
   └─────────────────────────────────────────────┘

4. Missing CSR_TIME Synchronization
   ┌─────────────────────────────────────────────┐
   │ CSR_TIME not incremented on every cycle     │
   │ Emulator "skips" some time steps            │
   │                                             │
   │ Result: Timer compare never triggered       │
   └─────────────────────────────────────────────┘

5. Read-Modify-Write on CSR_STIMECMP
   ┌─────────────────────────────────────────────┐
   │ OLD_VAL = read(CSR_STIMECMP)  // = ULONG_MAX│
   │ NEW_VAL = OLD_VAL | 0x1                     │
   │ write(CSR_STIMECMP, NEW_VAL)                │
   │                                             │
   │ But kernel does direct write, not RMW:      │
   │ write(CSR_STIMECMP, next_tval) // Direct    │
   └─────────────────────────────────────────────┘
```

## Emulator correctness

### Flw / FmvWX NaN-boxing is wrong

The RISC-V F extension requires that single-precision values stored in
double-precision registers be NaN-boxed: the upper 32 bits must be all
1s. The code does this:

gov64 |= uint64(0xffffffff) << 32

This ORs in the upper bits — but it should be setting the bits
unconditionally, which is fine here since v64 started as
zero-extended. However, the resulting 64-bit pattern is then
interpreted by the host as a double-precision float via
Float64frombits. That bit pattern (0xFFFFFFFF_xxxxxxxx) is a quiet NaN
in IEEE 754 double precision — meaning any subsequent double-precision
operation on this register will silently propagate NaN rather than
operating on the intended single-precision value. The register file
should store the raw bits instead, or FP operations must unwrap the
NaN box. Since FcvtWD just casts cpu.F[instr.Rs1] as float64, loading
a float32 via Flw and then converting with FcvtWD will produce
garbage.

### Div — signed overflow case is missing

The RISC-V spec mandates a special case: INT64_MIN / -1 must return
INT64_MIN (not trap, not undefined). Go's integer division will panic
with a runtime overflow here. The fix:

``` go
case isa.Div:
    rs1 := int64(cpu.X[instr.Rs1])
    rs2 := int64(cpu.X[instr.Rs2])
    if rs2 == 0 {
        cpu.X[instr.Rd] = ^uint64(0)
    } else if rs1 == math.MinInt64 && rs2 == -1 {
        cpu.X[instr.Rd] = uint64(math.MinInt64) // overflow case
    } else {
        cpu.X[instr.Rd] = uint64(rs1 / rs2)
    }
```

The same applies to `Divw` (`INT32_MIN / -1`) and `Rem`/`Remw`.

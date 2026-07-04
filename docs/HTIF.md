# Host Target Interface (HTIF)

This is low-level technical specification for the Host-Target
Interface (HTIF) protocol.

Because HTIF lacks an official, standalone PDF specification document,
this reference manual synthesizes the protocol constraints implemented
by the canonical Berkeley tools: the Spike simulator (riscv-isa-sim)
and the RISC-V Proxy Kernel (riscv-pk).

## 1. Base Register Mechanics

HTIF relies entirely on a shared-memory, polling-based FIFO
architecture using two volatile, naturally aligned, 64-bit
communication registers.

| Register | Direction     | Behavior                                        |
|----------|---------------|-------------------------------------------------|
| tohost   | Target → Host | Guest writes a command. Host writes 0 to ACK.   |
| fromhost | Host → Target | Host writes inputs. Guest clears fromhost to 0. |

## Crucial Synchronization Rules

* Spin-Locks: Before writing to tohost, the target must spin-lock
  until tohost == 0.
* Interlocking: The target must not issue a new tohost request if a
  previous response is still pending acknowledgment in fromhost (i.e.,
  fromhost != 0), unless explicitly authorized by a non-blocking
  device command.

## 2. Wire Format (Bit Layout)

Both tohost and fromhost packets pack commands, device IDs, and
payload metadata into a single 64-bit doubleword.

```
 63        56 55        48 47                                        0
+------------+------------+-------------------------------------------+
| Device ID  |  Command   |                  Payload                  |
+------------+------------+-------------------------------------------+
 [Bits 63:56] [Bits 55:48]              [Bits 47:0]
```

## Bitfield Extractor Definitions

* Device ID (tohost >> 56 & 0xFF): Maps the request to a physical or
  virtual hardware subsystem.
* Command (tohost >> 48 & 0xFF): Dictates the specific subsystem
  operation to execute.
* Payload (tohost & 0x00FFFFFFFFFFFFFF): Contains up to 48 bits of
  parameter raw data, arguments, or raw memory pointers.

## 3. Complete Command Dictionary

## Device 0x00: System Control & Proxy Syscalls

This device coordinates the target’s runtime lifecycle, software
assertions, and delegation of POSIX file/system requests to the host
kernel.

### Command 0x00: Execution Termination (Exit)

Signals that the guest process or micro-benchmark has completed.

* Target Request (tohost):
* Device ID: 0x00
   * Command: 0x00
   * Payload: Exit Code (48-bit unsigned integer)
     * Payload == 1 (0x1): Success / Pass. All assertions within the
       riscv-test binary passed.
     * Payload > 1: Failure. The payload represents
       (Failed_Test_Case_ID << 1) | 1. To recover the failing
       assertion ID, the host must right-shift the payload: Test_ID =
       payload >> 1.

     * Payload == 0: Explicit exit with code 0 (often treated as
       success depending on runtime framework).
   * Host Response (fromhost): None. The emulator terminates execution
     immediately.

### Command 0x01: POSIX Syscall Proxying

Delegates target system operations directly to the host operating
system.

* Target Request (tohost):
* Device ID: 0x00
   * Command: 0x01
   * Payload: Physical Memory Pointer pointing to a packed data block
     containing the syscall arguments.
* Host Response (fromhost):
* Device ID: 0x00
   * Command: 0x01
   * Payload: Syscall Return Value (typically 0 for success or a
     negative error code).

## Device 0x01: Console Character I/O

Provides a standard, basic line-buffered or raw character terminal
interface.

### Command 0x00: Read Character (Polled)

Requests characters from the host's standard input.

* Target Request (tohost):
* Device ID: 0x01
   * Command: 0x00
   * Payload: 0x000000000000 (Ignored)
* Host Response (fromhost):
* Device ID: 0x01
   * Command: 0x00
   * Payload: The 8-bit ASCII character data retrieved from stdin. If
     no character is present in the host buffer, the host leaves
     fromhost as 0 until a keystroke occurs (blocking mode) or returns
     an EOF marker dependent on implementation.

### Command 0x01: Write Character

Pushes a single ASCII byte out to the host's terminal screen.

* Target Request (tohost):
* Device ID: 0x01
   * Command: 0x01
   * Payload: Lower 8 bits contain the ASCII character code to be
     printed.
* Host Response (fromhost): None required. The host prints the
  character to stdout and immediately clears tohost to 0.

## 4. Deep Dive: Syscall Proxy Block Layout

When Device 0x00 Command 0x01 is invoked, the 48-bit payload field is
treated as a 64-bit physical address pointer (the upper 16 bits of the
address are assumed to be zeroed or sign-extended depending on target
architecture limits).  At that memory location, the guest target
creates an array of 64-bit slots containing the specific POSIX
arguments:

```
Address Ptr  +0x00: [ Syscall Number (uint64_t) ]
             +0x08: [ Argument 1     (uint64_t) ]
             +0x10: [ Argument 2     (uint64_t) ]
             +0x18: [ Argument 3     (uint64_t) ]
             +0x20: [ Argument 4     (uint64_t) ]
```

## Primary Proxy Syscall Numbers (RISC-V ABI)

The system call IDs inside the memory block follow the standard RISC-V
internal execution numbers, not host Linux architectures:

| Syscall Name | Number | Expected Target Block Structure        |
|--------------|--------|----------------------------------------|
| sys_write    | 64     | [64, fd, buffer_ptr, length]           |
| sys_read     | 63     | [63, fd, buffer_ptr, length]           |
| sys_openat   | 56     | [56, dirfd, filename_ptr, flags, mode] |
| sys_close    | 57     | [57, fd]                               |
| sys_fstat    | 80     | [80, fd, stat_struct_ptr]              |

## Example Host Implementation Logic for sys_write

When the host intercepts tohost with Device 0 / Command 1, it decodes
the pointer block:

   1. Read Block: Read memory starting at Payload Address.
   2. Identify Syscall: If Memory[Address] == 64, fetch parameters:
   * fd = Memory[Address + 8]
      * buf_addr = Memory[Address + 16]
      * len = Memory[Address + 24]
   3. Execute: The host safely reads len bytes from guest physical
      memory starting at buf_addr, writes it out to its own file
      descriptor matching fd, and captures the integer response bytes
      written.
   4. Respond: The host writes the return integer value back into the
      fromhost packet wrapper.

# Emulator Architectural Verification Checklist: NetBSD Compatibility Audit

This checklist isolates low-level micro-architectural requirements
mandatory for booting strict operating systems like NetBSD. It focuses
on Atomic Operations (AMO/LR/SC) and Page Table Entry (PTE) handling
within the Memory Management Unit (MMU).

---

## 1. Atomic Memory Operations & Reservation State Machine

NetBSD's `pmap_tlb` layer relies heavily on atomic loops to update
page directory entries concurrently across threads. If your
synchronization primitives or architectural reservation invalidations
leak state, thread synchronization structures (`ti->ti_lock`)
desynchronize.

### Core Reservation Variables

Ensure your processor struct explicitly isolates the reservation tracker:
* [ ] `reservation_address`: Tracks the *physical* address reserved by `lr`.
* [ ] `reservation_valid`: Boolean flag indicating if the reservation link is active.

### Conditional Store / Atomic Operations Verification Matrix

| Instruction         | Behavior to Verify                                                                                                                                                | Pass / Fail |
| :---                | :---                                                                                                                                                              | :---:       |
| **`lr.w` / `lr.d`** | Registers the target physical address in `reservation_address` and sets `reservation_valid = true`.                                                               | [ ]         |
| **`sc.w` / `sc.d`** | **Success:** If `reservation_valid == true` and access matches `reservation_address`, write data to memory, set `rd = 0`, then clear `reservation_valid = false`. | [ ]         |
| **`sc.w` / `sc.d`** | **Failure:** If `reservation_valid == false` or address mismatches, **abort write**, set `rd = 1`, and clear `reservation_valid = false`.                         | [ ]         |
| **`amoswap.w/.d`**  | Atomically fetches old value from memory into `rd` and stores new value from `rs2` into memory. Bypass regular `lr/sc` tracking variables.                        | [ ]         |
| **`amoadd.w/.d`**   | Atomically adds memory contents and `rs2`, storing the original value in `rd`. Must execute natively atomically in host Go loop.                                  | [ ]         |

### Mandatory Reservation Invalidation Triggers

The RISC-V specification dictates that reservations are fragile. To
guarantee NetBSD compatibility, your emulator **must intercept and
invalidate** the active reservation on any of the following
architectural events:

- [ ] **Traps & Exceptions:** Any trap transition (`ecall`, `ebreak`, page faults, load/store misaligned traps) routing to `mtvec` or `stvec` sets `reservation_valid = false`.
- [ ] **Privilege Transitions:** Executing an `sret` or `mret` sequence explicitly clears `reservation_valid = false`.
- [ ] **Context Fences:** Executing a `sfence.vma` instruction clears `reservation_valid = false`.
- [ ] **Interfering Stores:** If *any* local write (or external DMA write if implemented) targets the bytes covered by `reservation_address`, clear `reservation_valid = false`.

---

## 2. Page Table Entry (PTE) Attribute Flags Handling

NetBSD validates individual page table entry attributes stringently
compared to early Linux variants. Shortcuts in access fault generation
will cause silent looping or panic failures during early virtual
memory allocation maps.


```
Sv39 Page Table Entry (PTE) Bit Fields:
63      54 53                            28 27      19 18      10 9   8 7 6 5 4 3 2 1 0
+----------+--------------------------------+----------+----------+-----+---+-+-+-+-+-+-+
| Reserved |             PPN[2]             |  PPN[1]  |  PPN[0]  | RSW | D | A |G |U |X|W|R|V|
+----------+--------------------------------+----------+----------+-----+---+-+-+-+-+-+-+
```

### Flag Rules Cheat Sheet
* **Valid (`V`):** Bit 0. If `0`, all other bits are ignored; immediately throw a Page Fault.
* **Readable (`R`), Writable (`W`), Executable (`X`):** Bits 1, 2, and 3.
  * If `R=0, W=1`: **Strictly Invalid Architecture State.** Must immediately raise a Page Fault.
  * If `R=0, W=0, X=0`: Pointer to next level page table.
  * Otherwise: Leaf page table entry.
* **User (`U`):** Bit 4. S-mode cannot access `U=1` pages unless `sstatus.SUM = 1`.
* **Accessed (`A`):** Bit 6. Page has been read, written, or fetched since the last clear.
* **Dirty (`D`):** Bit 7. Page has been modified/written to since the last clear.

---

## 3. Detailed Technical Breakdown: MMU, TLB, and A/D Bit Tracking

The interaction between physical page table memory, your internal
Translation Lookaside Buffer (TLB), and the updating of the `A` and
`D` flags is critical. If your TLB caches a page map without recording
flag mutations correctly, the software-side OS state machine will
desynchronize.

### Architectural TLB & Flag Lifecycle Loop

```
                 +---------------------------+
                 |   Virtual Address Match   |
                 +-------------+-------------+
                               |
                               v
                 +-------------+-------------+
                 |      Is Entry in TLB?     |
                 +------+--------------+-----+
                        |              |
                YES     |              | NO
                        v              v
           +------------+----+   +-----+---------------------+
           | Read Cached PTE |   | Perform Hardware Page Walk|
           +------------+----+   +-----+---------------------+
                        |              |
                        |              | (Sets 'A' on Memory Read)
                        v              v
           +------------+----+   +-----+---------------------+
           | Is it a WRITE?  |   | Populate/Inject into TLB  |
           +------+------+---+   +-------------+-------------+
                  |      |                     |
         YES      |      | NO                  |
                  v      |                     |
   +--------------+---+  |                     |
   |  Is PTE.D == 1?  |  |                     |
   +------+------+----+  |                     |
          |      |       |                     |
   NO     |      | YES   |                     |
          v      +-------+---------------------+

+-----------+-----------+  |
| 1. Clear TLB Entry    |  |
| 2. Write PTE.D=1 to RAM| |
| 3. Re-fetch to TLB    |  |
+-----------+-----------+  |
|              |
v              v
+------+--------------+---+
| Commit Memory Access    |
+-------------------------+
```

### Step-by-Step Handling Implementation Mechanics

#### Phase A: Hardware Page Walk (On TLB Miss)

When a virtual translation completely misses your internal TLB cache structure:
1. Traverse down through Level 2 $\rightarrow$ Level 1 $\rightarrow$ Level 0 descriptors inside your hardware page walker loop.
2. Find the terminal leaf entry. Verify compliance (`V=1`, and `R/W/X` structure validity).
3. **Handle Accessed (`A`) Bit:**
   * Regardless of whether the access is a read, write, or instruction execute fetch, the `A` bit **must** be verified.
   * If `PTE.A == 0`, you must dynamically modify the byte directly inside the guest's physical RAM layout, setting `PTE.A = 1`.
   * *Implementation Note:* This modification must bypass your regular MMU caching rules to ensure it updates immediately in system physical RAM.

#### Phase B: Access Execution & TLB Flag Updates (Hit Optimization)

Operating systems cache translations in the TLB to maximize processing
bandwidth. Your emulator must track flag conditions on active TLB hits
precisely during access:

* [ ] **Read Access (`Load` instructions or Instruction Fetch):**
  * Check the cached TLB descriptor. If the page table memory lookup completed successfully, the `A` bit was already validated/set in physical RAM during Phase A.
  * Execute the access directly from the cached physical page address mapping inside your TLB.
* [ ] **Write Access (`Store` instructions or Atomic Mutations):**
  * Find the corresponding target translation inside your TLB array.
  * **Verify Writable Permission:** Check that `PTE.W == 1`. If not, generate a Store Page Fault.
  * **Evaluate Dirty State:** Inspect the cached entry flag field. If `PTE.D == 1`, write access is clear. You can safely commit the bytes directly to physical memory locations immediately.
  * **Execute Dirty (`D`) Bit Injection on TLB Hit:**
    If `PTE.D == 0` when checked inside your hit tracking logic, you must handle the entry mutation exactly like this to prevent kernel tracking structures from losing visibility:
    1. **Evict/Invalidate** this specific virtual mapping target address frame entirely from your internal emulator TLB array.
    2. Reach directly into the guest's underlying physical RAM layout at the leaf descriptor’s base address.
    3. Modify the memory byte value explicitly to mark `PTE.D = 1` (and ensure `PTE.A = 1` concurrently).
    4. Re-populate/Inject the modified leaf descriptor layout back into your TLB cache matrix structure with the `D` bit now reporting active state.
    5. Allow the store sequence to resume and finalize its memory commit pipeline cleanly.

*Failure to completely flush the old `D=0` translation from the
internal emulator TLB array when updating guest RAM results in
subsequent store updates continuing to hitting a cached `D=0` state
entry, causing NetBSD's `uvm` pmap manager to deadlock or trigger an
invariant check crash.*

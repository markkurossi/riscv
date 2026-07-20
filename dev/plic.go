//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package dev

import (
	"fmt"
	"log"
	"sync"

	"github.com/markkurossi/riscv/isa"
)

const (
	// Maximum number of interrupt sources supported.
	PLICMaxInterrupts = 63

	// The maximum number of contexts supported by the PLIC
	// specification.
	PLICMaxContexts = 15872

	// Size of the PLIC memory map.
	PLICSize = 0x400000
)

// PLICContext holds the registers dedicated to a specific privilege
// context.
type PLICContext struct {
	// Priority threshold register (e.g., 0x0c200000)
	Threshold uint32

	// Claim/Complete register  (e.g., 0x0c200004)
	// XXX unused
	ClaimXXX uint32
}

// Gateway converts interrupts signals from source to pending
// interrupts at PLIC core.
type Gateway struct {
	priority uint32
	asserted bool
	inflight bool
}

type PLIC struct {
	Harts         []isa.Hart
	Start         uint64
	End           uint64
	MaxInterrupts uint32
	IRQs          map[uint32]string

	m           sync.Mutex
	numContexts int

	// Interrupt gateways for each interrupt source. Gateway 0 is
	// reserved/unused. The priority field is mapped to offsets
	// 0x000000 to 0x000080 of the PLIC memory.
	gateways [PLICMaxInterrupts + 1]Gateway

	// 0x001000 to 0x001004: Pending bitmask (Read-Only to the guest).
	pending uint64

	// 0x002000 to 0x002084: Enable bitmasks for each context.  Each
	// context has 64 bits to cover 63 interrupt sources.
	enables []uint64

	// 0x200000 onwards: Control states grouped cleanly per context.
	contexts []PLICContext
}

func NewPLIC(harts []isa.Hart, start uint64) *PLIC {
	numContexts := len(harts) * 2
	if numContexts > PLICMaxContexts {
		panic("maximum number of contexts exceeded")
	}
	return &PLIC{
		Harts:         harts,
		Start:         start,
		End:           start + PLICSize,
		MaxInterrupts: PLICMaxInterrupts,
		IRQs:          make(map[uint32]string),
		numContexts:   numContexts,
		enables:       make([]uint64, numContexts),
		contexts:      make([]PLICContext, numContexts),
	}
}

func (plic *PLIC) Halt() error {
	return nil
}

func (plic *PLIC) Contains(paddr uint64) bool {
	return paddr >= plic.Start && paddr < plic.End
}

func (plic *PLIC) Load8(paddr uint64) (uint8, error) {
	if paddr < plic.Start {
		return 0, plic.Harts[0].Trap(isa.CauseStorePageFault, paddr, nil)
	}
	log.Printf("PLIC: Load8(0x%x)", paddr)
	return 0, nil
}

func (plic *PLIC) Load16(paddr uint64) (uint16, error) {
	log.Printf("PLIC: Load16(0x%x)", paddr)
	return 0, nil
}

func (plic *PLIC) Load32(paddr uint64) (uint32, error) {
	if paddr < plic.Start {
		return 0, plic.Harts[0].Trap(isa.CauseLoadPageFault, paddr, nil)
	}

	plic.m.Lock()
	defer plic.m.Unlock()

	offset := paddr - plic.Start

	switch {
	case offset >= 0x000000 && offset <= 0x000080:
		return plic.gateways[offset/4].priority, nil

	case offset == 0x001000:
		return uint32(plic.pending), nil

	case offset == 0x001004:
		return uint32(plic.pending >> 32), nil

	case offset >= 0x002000 && offset <= 0x002084:
		contextID := int((offset - 0x2000) / 0x80)
		if contextID < len(plic.enables) {
			word := int((offset - 0x2000) % 0x80)
			if word == 0 {
				return uint32(plic.enables[contextID]), nil
			} else {
				return uint32(plic.enables[contextID] >> 32), nil
			}
		}

	case offset >= 0x200000:
		contextOffset := offset - 0x200000
		contextID := int(contextOffset / 0x1000)
		regRegister := contextOffset % 0x1000

		if contextID < len(plic.contexts) {
			if regRegister == 0x0 {
				return plic.contexts[contextID].Threshold, nil
			} else if regRegister == 0x4 {
				// Reading claim register evaluates pending IRQs and pulls high
				return plic.claimInterrupt(contextID), nil
			}
		}
	}

	return 0, nil
}

func (plic *PLIC) Load64(paddr uint64) (uint64, error) {
	log.Printf("PLIC: Load64(0x%x)", paddr)
	return 0, nil
}

func (plic *PLIC) Store8(paddr uint64, v uint8) error {
	if paddr < plic.Start {
		return plic.Harts[0].Trap(isa.CauseStorePageFault, paddr, nil)
	}
	log.Printf("PLIC: 0x%x = 0x%x", paddr, v)
	return nil
}

func (plic *PLIC) Store16(paddr uint64, v uint16) error {
	log.Printf("PLIC: 0x%x = 0x%02x", paddr, v)
	return nil
}

func (plic *PLIC) Store32(paddr uint64, val uint32) error {
	if paddr < plic.Start {
		return plic.Harts[0].Trap(isa.CauseStorePageFault, paddr, nil)
	}

	plic.m.Lock()
	defer plic.m.Unlock()

	offset := paddr - plic.Start

	switch {
	// 1. Interrupt Source Priorities (0x000000 - 0x000080)
	case offset >= 0x000000 && offset <= 0x000080:
		sourceID := offset / 4
		if sourceID <= PLICMaxInterrupts {
			// Only 3 bits of priority (0-7).
			plic.gateways[sourceID].priority = uint32(val & 0x7)
		}

	// 2. Interrupt Enables (0x002000 - 0x002084)
	case offset >= 0x002000 && offset <= 0x002084:
		// Stride is 0x80 bytes per context for enable bits
		contextID := int((offset - 0x2000) / 0x80)
		if contextID < len(plic.enables) {
			word := (offset - 0x2000) % 0x80
			old := plic.enables[contextID]
			if word == 0 {
				plic.enables[contextID] = old&0xffffffff00000000 | uint64(val)
			} else {
				plic.enables[contextID] = old&0xffffffff | uint64(val)<<32
			}

			var changed string

			for i := 0; i < 64; i++ {
				bit := uint64(1) << i
				os := old & bit
				ns := plic.enables[contextID] & bit
				if os != ns {
					if len(changed) > 0 {
						changed += ", "
					}
					if ns != 0 {
						changed += "+"
					} else {
						changed += "-"
					}
					changed += plic.IRQs[uint32(i)]
				}
			}
		}

	// 3. Priority Thresholds & Claim/Complete Blocks (0x200000 onwards)
	case offset >= 0x200000:
		// Stride is 0x1000 (4KB page alignment) per context target
		contextOffset := offset - 0x200000
		contextID := int(contextOffset / 0x1000)
		regRegister := contextOffset % 0x1000

		if contextID < plic.numContexts {
			if regRegister == 0x0 { // Threshold Register
				plic.contexts[contextID].Threshold = uint32(val & 0x7)
			} else if regRegister == 0x4 { // Claim / Complete Register

				// Writing to claim means the guest OS finished
				// handling an IRQ line.
				plic.completeInterrupt(contextID, uint32(val))
			}
		}
	}

	return nil
}

func (plic *PLIC) Store64(paddr uint64, v uint64) error {
	log.Printf("PLIC: 0x%x = 0x%08x", paddr, v)
	return nil
}

func (plic *PLIC) SetInterruptRequest(irq uint32, set bool) {
	plic.m.Lock()
	defer plic.m.Unlock()

	old := plic.gateways[irq].asserted
	plic.gateways[irq].asserted = set

	if !old && set && !plic.gateways[irq].inflight {
		plic.pending |= 1 << irq
	}

	plic.reevaluateInterrupts()
}

func (plic *PLIC) claimInterrupt(contextID int) uint32 {

	var highestPriority uint32 = 0
	var claimedSource uint32 = 0

	// Walk through all configured interrupt sources.
	for sourceID := uint32(1); sourceID <= PLICMaxInterrupts; sourceID++ {
		// Check if the source is enabled for this context and is
		// actively pending.
		isPending := (plic.pending & (1 << sourceID)) != 0
		isEnabled := (plic.enables[contextID] & (1 << sourceID)) != 0

		if isPending && isEnabled {
			priority := plic.gateways[sourceID].priority
			// Only consider it if it strictly exceeds the context's
			// current threshold.
			if priority > plic.contexts[contextID].Threshold {
				if priority > highestPriority {
					highestPriority = priority
					claimedSource = sourceID
				}
			}
		}
	}

	if claimedSource != 0 {
		// Hardware handshake: clear the pending state atomically upon
		// claiming.
		plic.pending &^= (1 << claimedSource)
		plic.gateways[claimedSource].inflight = true
		plic.reevaluateInterrupts()
	}

	return claimedSource
}

func (plic *PLIC) completeInterrupt(contextID int, sourceID uint32) {
	if sourceID == 0 || sourceID > PLICMaxInterrupts {
		return
	}
	if !plic.gateways[sourceID].inflight {
		return
	}

	plic.gateways[sourceID].inflight = false
	if plic.gateways[sourceID].asserted {
		plic.pending |= 1 << sourceID
	}

	plic.reevaluateInterrupts()
}

func (plic *PLIC) reevaluateInterrupts() {
	// Loop over all contexts.
	for context := 0; context < plic.numContexts; context++ {
		var signaled uint32
		for sourceID := uint32(1); sourceID <= PLICMaxInterrupts; sourceID++ {
			isPending := (plic.pending & (1 << sourceID)) != 0
			isEnabled := (plic.enables[context] & (1 << sourceID)) != 0

			if isPending && isEnabled &&
				plic.gateways[sourceID].priority >
					plic.contexts[context].Threshold {
				signaled = sourceID
				break
			}
		}
		hart := context / 2
		var interrupt uint64
		if context%2 == 0 {
			interrupt = isa.IntMEIP
		} else {
			interrupt = isa.IntSEIP
		}
		if signaled > 0 {
			plic.Harts[hart].SetInterrupt(interrupt)
		} else {
			plic.Harts[hart].ClearInterrupt(interrupt)
		}
	}
}

func (plic *PLIC) irq(irq uint32) string {
	return fmt.Sprintf("%v[%v]", irq, plic.IRQs[irq])
}

//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"fmt"

	"github.com/markkurossi/riscv/isa"
)

const (
	// Maximum number of interrupt sources supported.
	MaxInterrupts = 32

	// Context 0 = M-Mode Hart 0, Context 1 = S-Mode Hart 0.
	MaxContexts = 2
)

// PlicContext holds the registers dedicated to a specific privilege
// context.
type PlicContext struct {
	// Priority threshold register (e.g., 0x0c200000)
	Threshold uint32

	// Claim/Complete register  (e.g., 0x0c200004)
	Claim uint32
}

type PLIC struct {
	Hart  isa.Hart
	Start uint64
	End   uint64

	// 0x000000 to 0x000080: Priorities for each interrupt source
	// Source 0 is reserved/unused (index 0). Index 1-32 map to
	// sources 1-32.
	Priorities [MaxInterrupts + 1]uint32

	// 0x001000 to 0x001004: Pending bitmask (Read-Only to the guest).
	Pending uint32

	// 0x002000 to 0x002084: Enable bitmasks for each context.
	//
	// Context 0 (M-mode) uses index 0, Context 1 (S-mode) uses index 1.
	// Each context has 32 bits (1 word) to cover 32 interrupt sources.
	Enables [MaxContexts]uint32

	// 0x200000 onwards: Control states grouped cleanly per context.
	Contexts [MaxContexts]PlicContext
}

func (plic *PLIC) Halt() error {
	return nil
}

func (plic *PLIC) Contains(paddr uint64) bool {
	return paddr >= plic.Start && paddr < plic.End
}

func (plic *PLIC) Load8(paddr uint64) (uint8, error) {
	if paddr < plic.Start {
		return 0, plic.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
	}
	return 0, nil
}

func (plic *PLIC) Load16(paddr uint64) (uint16, error) {
	return 0, nil
}

func (plic *PLIC) Load32(paddr uint64) (uint32, error) {
	if paddr < plic.Start {
		return 0, plic.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
	}

	offset := paddr - plic.Start

	switch {
	case offset >= 0x000004 && offset <= 0x000080:
		return plic.Priorities[offset/4], nil

	case offset == 0x001000:
		return plic.Pending, nil

	case offset >= 0x002000 && offset <= 0x002084:
		contextID := (offset - 0x2000) / 0x80
		if contextID < MaxContexts {
			return plic.Enables[contextID], nil
		}

	case offset >= 0x200000:
		contextOffset := offset - 0x200000
		contextID := uint32(contextOffset / 0x1000)
		regRegister := contextOffset % 0x1000

		if contextID < MaxContexts {
			if regRegister == 0x0 {
				return plic.Contexts[contextID].Threshold, nil
			} else if regRegister == 0x4 {
				// Reading claim register evaluates pending IRQs and pulls high
				return plic.ClaimInterrupt(contextID), nil
			}
		}
	}

	return 0, nil
}

func (plic *PLIC) Load64(paddr uint64) (uint64, error) {
	return 0, nil
}

func (plic *PLIC) Store8(paddr, v uint64) error {
	if paddr < plic.Start {
		return plic.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
	}
	fmt.Printf("PLIC: 0x%x = 0x%x\r\n", paddr, v)
	return nil
}

func (plic *PLIC) Store16(paddr, v uint64) error {
	fmt.Printf("PLIC: 0x%x = 0x%02x\r\n", paddr, v)
	return nil
}

func (plic *PLIC) Store32(paddr uint64, val uint64) error {
	if paddr < plic.Start {
		return plic.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
	}

	offset := paddr - plic.Start

	switch {
	// 1. Interrupt Source Priorities (0x000004 - 0x000080)
	case offset >= 0x000004 && offset <= 0x000080:
		sourceID := offset / 4
		if sourceID <= MaxInterrupts {
			// Only 3 bits of priority (0-7).
			plic.Priorities[sourceID] = uint32(val & 0x7)
		}

	// 2. Interrupt Enables (0x002000 - 0x002084)
	case offset >= 0x002000 && offset <= 0x002084:
		// Stride is 0x80 bytes per context for enable bits
		contextID := (offset - 0x2000) / 0x80
		if contextID < MaxContexts {
			plic.Enables[contextID] = uint32(val)
		}

	// 3. Priority Thresholds & Claim/Complete Blocks (0x200000 onwards)
	case offset >= 0x200000:
		// Stride is 0x1000 (4KB page alignment) per context target
		contextOffset := offset - 0x200000
		contextID := uint32(contextOffset / 0x1000)
		regRegister := contextOffset % 0x1000

		if contextID < MaxContexts {
			if regRegister == 0x0 { // Threshold Register
				plic.Contexts[contextID].Threshold = uint32(val & 0x7)
			} else if regRegister == 0x4 { // Claim / Complete Register

				// Writing to claim means the guest OS finished
				// handling an IRQ line.
				plic.CompleteInterrupt(contextID, uint32(val))
			}
		}
	}

	return nil
}

func (plic *PLIC) Store64(paddr, v uint64) error {
	fmt.Printf("PLIC: 0x%x = 0x%08x\r\n", paddr, v)
	return nil
}

func (plic *PLIC) ClaimInterrupt(contextID uint32) uint32 {
	var highestPriority uint32 = 0
	var claimedSource uint32 = 0

	// Walk through all configured interrupt sources.
	for sourceID := uint32(1); sourceID <= MaxInterrupts; sourceID++ {
		// Check if the source is enabled for this context AND is
		// actively pending.
		isPending := (plic.Pending & (1 << sourceID)) != 0
		isEnabled := (plic.Enables[contextID] & (1 << sourceID)) != 0

		if isPending && isEnabled {
			priority := plic.Priorities[sourceID]
			// Only consider it if it strictly exceeds the context's
			// current threshold.
			if priority > plic.Contexts[contextID].Threshold {
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
		plic.Pending &^= (1 << claimedSource)
		plic.ReevaluateInterrupts()
	}

	return claimedSource
}

func (plic *PLIC) CompleteInterrupt(contextID uint32, sourceID uint32) {
	if sourceID == 0 || sourceID > MaxInterrupts {
		return // Invalid source ID complete request
	}

	// In physical silicon, this unmasks the target line inside the
	// PLIC gateway routing.  Now that the handler loop is completely
	// clear, we re-evaluate if any other enabled interrupts are
	// sitting in the pipeline waiting for their turn.
	plic.ReevaluateInterrupts()
}

func (plic *PLIC) ReevaluateInterrupts() {
	// 1. Re-evaluate M-Mode interrupts (Context 0)
	mModeSignaled := false
	for sourceID := uint32(1); sourceID <= MaxInterrupts; sourceID++ {
		isPending := (plic.Pending & (1 << sourceID)) != 0
		isEnabled := (plic.Enables[0] & (1 << sourceID)) != 0
		if isPending && isEnabled && plic.Priorities[sourceID] > plic.Contexts[0].Threshold {
			mModeSignaled = true
			break
		}
	}
	if mModeSignaled {
		plic.Hart.SetInterrupt(isa.IntMEIP)
	} else {
		plic.Hart.ClearInterrupt(isa.IntMEIP)
	}

	// 2. Re-evaluate S-Mode interrupts (Context 1)
	sModeSignaled := false
	for sourceID := uint32(1); sourceID <= MaxInterrupts; sourceID++ {
		isPending := (plic.Pending & (1 << sourceID)) != 0
		isEnabled := (plic.Enables[1] & (1 << sourceID)) != 0
		if isPending && isEnabled && plic.Priorities[sourceID] > plic.Contexts[1].Threshold {
			sModeSignaled = true
			break
		}
	}
	if sModeSignaled {
		plic.Hart.SetInterrupt(isa.IntSEIP)
	} else {
		plic.Hart.ClearInterrupt(isa.IntSEIP)
	}
}

//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package virtio

import (
	"crypto/rand"
	"fmt"

	"github.com/markkurossi/riscv/dev"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/logger"
	"github.com/markkurossi/riscv/memory"
)

const (
	RngDeviceID = 4
	RngSize     = 0x100
)

type Rng struct {
	MMIO

	// Statistics.
	stRead uint64
}

func NewRng(hart isa.Hart, start uint64, plic *dev.PLIC, irq uint32,
	mem *memory.Memory) *Rng {

	rng := &Rng{
		MMIO: MMIO{
			Log: logger.Log{
				Name:  "virtio-rng",
				Level: logger.Error,
			},
			DeviceID: RngDeviceID,
			Hart:     hart,
			Start:    start,
			End:      start + RngSize,
			Plic:     plic,
			IRQ:      irq,
			Mem:      mem,
		},
	}

	rng.Init(1)
	rng.MMIO.Handler = rng

	return rng
}

// Reset implements Handler.Reset.
func (rng *Rng) Reset() error {
	return nil
}

// DeviceStats implements Handler.DeviceStats
func (rng *Rng) DeviceStats() {
	fmt.Printf("%v: read    : %v\n", rng.Name, FileSize(rng.stRead))
}

// ExecuteDescriptorChain implements Handler.ExecuteDescriptorChain.
func (rng *Rng) ExecuteDescriptorChain(vq *Queue, idx uint16) (uint32, error) {
	rng.Debugf("chain: idx=%v", idx)

	var transferred uint32

	for {
		req, err := vq.loadDesc(idx)
		if err != nil {
			return 0, err
		}
		rng.Debugf("req: %v", req)

		if req.Flags&VIRTQ_DESC_F_WRITE != 0 {
			buf, err := rng.guestData(req.Addr, uint64(req.Len))
			if err != nil {
				rng.Errorf("guestData(%v,%v) failed: %v",
					req.Addr, req.Len, err)
				return 0, err
			}
			n, err := rand.Read(buf)
			if err != nil {
				rng.Errorf("rand.Read(%v) failed: %v", req.Len, err)
				return 0, err
			} else {
				rng.stRead += uint64(n)
			}

			rng.Infof("idx=%v, len=%v, addr=%x, tx=%v",
				idx, req.Len, req.Addr, n)

			transferred += uint32(n)
		}

		if req.Flags&VIRTQ_DESC_F_NEXT == 0 {
			break
		}
		idx = req.Next
	}

	rng.Debugf("transferred: %v", transferred)

	return transferred, nil
}

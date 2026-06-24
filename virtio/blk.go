//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package virtio

//lint:file-ignore ST1003 to match the C coding style for constants.

import (
	"fmt"
	"os"

	"github.com/markkurossi/riscv/dev"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/logger"
	"github.com/markkurossi/riscv/memory"
)

const (
	BlkDeviceID = 0x2
	BlkSize     = 4096
)

type Blk struct {
	MMIO
	File     *os.File
	Readonly bool
	fileInfo os.FileInfo
	id       []byte

	// Statistics.
	stRead  uint64
	stWrote uint64
}

func NewBlk(hart isa.Hart, start uint64, plic *dev.PLIC, irq uint32,
	mem *memory.Memory, file *os.File) *Blk {

	blk := &Blk{
		MMIO: MMIO{
			Log: logger.Log{
				Name:  "virtio-blk",
				Level: logger.Error,
			},
			DeviceID: BlkDeviceID,
			Features: 1 << VIRTIO_BLK_F_SEG_MAX,
			Hart:     hart,
			Start:    start,
			End:      start + BlkSize,
			Plic:     plic,
			IRQ:      irq,
			Mem:      mem,
		},
		File: file,
	}
	blk.Init(1)
	blk.MMIO.Handler = blk

	return blk
}

func (blk *Blk) SetID(id string) {
	blk.id = []byte(id)
	blk.id = append(blk.id, 0)
}

// Reset implements Handler.Reset.
func (blk *Blk) Reset() error {
	return nil
}

// DeviceStats implements Handler.DeviceStats
func (blk *Blk) DeviceStats() {
	fmt.Printf("%v: read  : %v\n", blk.Name, FileSize(blk.stRead))
	fmt.Printf("%v: wrote : %v\n", blk.Name, FileSize(blk.stWrote))
}

// ExecuteDescriptorChain implements Handler.ExecuteDescriptorChain.
func (blk *Blk) ExecuteDescriptorChain(vq *Queue, idx uint16) (uint32, error) {
	blk.Debugf("chain: idx=%v", idx)

	req, err := vq.loadDesc(idx)
	if err != nil {
		return 0, err
	}
	if req.Len != 16 || req.Flags&VIRTQ_DESC_F_NEXT == 0 {
		return 0, fmt.Errorf("invalid blk header: %v", blk)
	}
	t, err := blk.readGuestUint32(req.Addr)
	if err != nil {
		return 0, err
	}
	sector, err := blk.readGuestUint64(req.Addr + 8)
	if err != nil {
		return 0, err
	}
	blk.Debugf("req header : %v\n", req)
	blk.Debugf(" - type    : %v\n", blkTypeString(t))
	blk.Debugf(" - sector  : %v\n", sector)

	fileOffset := int64(sector) * 512

	var opStatus uint8 = VIRTIO_BLK_S_OK
	var statusSeen bool
	var transferred uint32

	// Process data and status blocks.
	for req.Flags&VIRTQ_DESC_F_NEXT != 0 {
		// Read the next block.
		req, err = vq.loadDesc(req.Next)
		if err != nil {
			return 0, err
		}
		if req.Flags&VIRTQ_DESC_F_NEXT != 0 {
			// Data block, handle request type.
			addr := req.Addr

			switch t {
			case VIRTIO_BLK_T_IN:
				buf, err := blk.guestData(addr, uint64(req.Len))
				if err != nil {
					blk.Errorf("guestData(%v,%v) failed: %v",
						addr, req.Len, err)
					opStatus = VIRTIO_BLK_S_IOERR
					continue
				}
				n, err := blk.File.ReadAt(buf, fileOffset)
				if err != nil {
					blk.Errorf("read failed from host file offset %d: %v",
						fileOffset, err)
					opStatus = VIRTIO_BLK_S_IOERR
				} else {
					blk.Infof("read: idx=%v, len=%v, addr=%x, tx=%v",
						idx, req.Len, addr, n)
					blk.stRead += uint64(n)
				}
				fileOffset += int64(n)
				transferred += uint32(n)

			case VIRTIO_BLK_T_OUT:
				if blk.Readonly {
					// Even if we don't write the data, transfer the
					// bytes to /dev/null so that this descriptor
					// chain completes.
					transferred += req.Len
					opStatus = VIRTIO_BLK_S_UNSUPP
				} else {
					buf, err := blk.guestData(addr, uint64(req.Len))
					if err != nil {
						blk.Errorf("guestData(%v,%v) failed: %v",
							addr, req.Len, err)
						opStatus = VIRTIO_BLK_S_IOERR
						continue
					}
					n, err := blk.File.WriteAt(buf, fileOffset)
					if err != nil {
						blk.Errorf("write failed to host file offset %d: %v",
							fileOffset, err)
						opStatus = VIRTIO_BLK_S_IOERR
					} else {
						blk.Infof("write: idx=%v, len=%v, addr=%x, tx=%v",
							idx, req.Len, addr, n)
						blk.stWrote += uint64(n)
					}
					fileOffset += int64(n)
					transferred += uint32(n)
				}

			case VIRTIO_BLK_T_GET_ID:
				buf, err := blk.guestData(addr, uint64(req.Len))
				if err != nil {
					blk.Errorf("guestData(%v,%v) failed: %v",
						addr, req.Len, err)
					opStatus = VIRTIO_BLK_S_IOERR
					continue
				}
				n := copy(buf, blk.id)
				blk.Infof("id: idx=%v, len=%v, addr=%x, tx=%v",
					idx, req.Len, addr, n)
				transferred += uint32(n)

			default:
				blk.Errorf("type %v not supported", blkTypeString(t))
				opStatus = VIRTIO_BLK_S_UNSUPP
			}

		} else {
			// Status block.
			statusSeen = true
			if req.Len != 1 {
				return 0, fmt.Errorf("invalid blk status: %v", blk)
			}
			err = blk.writeGuestUint8(req.Addr, opStatus)
			if err != nil {
				return 0, err
			}
		}
	}
	if !statusSeen {
		return 0, fmt.Errorf("invalid chain: no status block")
	}

	blk.Debugf("transferred: %v\n", transferred)
	blk.Debugf("req status : %v\n", opStatus)

	return transferred, nil
}

const (
	VIRTIO_BLK_T_IN           = 0
	VIRTIO_BLK_T_OUT          = 1
	VIRTIO_BLK_T_FLUSH        = 4
	VIRTIO_BLK_T_GET_ID       = 8
	VIRTIO_BLK_T_GET_LIFETIME = 10
	VIRTIO_BLK_T_DISCARD      = 11
	VIRTIO_BLK_T_WRITE_ZEROES = 13
	VIRTIO_BLK_T_SECURE_ERASE = 14
)

var blkTypes = map[uint32]string{
	VIRTIO_BLK_T_IN:           "read",
	VIRTIO_BLK_T_OUT:          "write",
	VIRTIO_BLK_T_FLUSH:        "flush",
	VIRTIO_BLK_T_GET_ID:       "get-id",
	VIRTIO_BLK_T_GET_LIFETIME: "get-lifetime",
	VIRTIO_BLK_T_DISCARD:      "discard",
	VIRTIO_BLK_T_WRITE_ZEROES: "write-zeroes",
	VIRTIO_BLK_T_SECURE_ERASE: "secure-erase",
}

func blkTypeString(t uint32) string {
	name, ok := blkTypes[t]
	if ok {
		return name
	}
	return fmt.Sprintf("{type %d}", t)
}

const (
	VIRTIO_BLK_S_OK     = 0
	VIRTIO_BLK_S_IOERR  = 1
	VIRTIO_BLK_S_UNSUPP = 2
)

const (
	VIRTIO_BLK_F_SIZE_MAX     = 1
	VIRTIO_BLK_F_SEG_MAX      = 2
	VIRTIO_BLK_F_GEOMETRY     = 4
	VIRTIO_BLK_F_RO           = 5
	VIRTIO_BLK_F_BLK_SIZE     = 6
	VIRTIO_BLK_F_FLUSH        = 9
	VIRTIO_BLK_F_TOPOLOGY     = 10
	VIRTIO_BLK_F_CONFIG_WCE   = 11
	VIRTIO_BLK_F_MQ           = 12
	VIRTIO_BLK_F_DISCARD      = 13
	VIRTIO_BLK_F_WRITE_ZEROES = 14
	VIRTIO_BLK_F_LIFETIME     = 15
	VIRTIO_BLK_F_SECURE_ERASE = 16
	VIRTIO_BLK_F_ZONED        = 17
)

var blkRegs = map[uint64]string{
	0x100: "CapacityLow",
	0x104: "CapacityHigh",
	0x108: "SizeMax",
	0x10c: "SeqMax",
}

func (blk *Blk) Load32(paddr uint64) (uint32, error) {
	offset := paddr - blk.Start

	reg, ok := blkRegs[offset]
	if ok {
		blk.Debugf("Load32(%v[0x%03x])", reg, offset)
	}

	switch offset {

	// 5.2.4 Device configuration layout at offset 0x100.

	case 0x100: // Disk size in sectors, low
		return uint32(blk.size()), nil

	case 0x104: // Disk size in sectors, high
		return uint32(blk.size() >> 32), nil

	case 0x108: // size_max
		return 4096, nil

	case 0x10c: // seg_max
		return queueNumMax - 2, nil

	default:
		return blk.MMIO.Load32(paddr)
	}
}

func (blk *Blk) size() uint64 {
	var err error

	if blk.fileInfo == nil {
		blk.fileInfo, err = blk.File.Stat()
		if err != nil {
			blk.Errorf("failed to stat image: %v", err)
			return 0
		}
	}
	return uint64((blk.fileInfo.Size() + 511) / 512)
}

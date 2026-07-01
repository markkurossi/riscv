//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package virtio

//lint:file-ignore ST1003 to match the C coding style for constants.

import (
	"encoding/hex"
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"log"
	"os"
	"runtime"
	"sync"

	"github.com/markkurossi/riscv/dev"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/logger"
	"github.com/markkurossi/riscv/memory"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

func init() {
	runtime.LockOSThread()
}

const (
	GPUDeviceID = 16
	GPUSize     = 4096
)

// Feature bits.
const (
	// Virgl 3D mode is supported.
	VIRTIO_GPU_F_VIRGL = 0

	// EDID is supported.
	VIRTIO_GPU_F_EDID = 1

	// Assigning resources UUIDs for export to other virtio devices is
	// supported.
	VIRTIO_GPU_F_RESOURCE_UUID = 2

	// Creating and using size-based blob resources is supported.
	VIRTIO_GPU_F_RESOURCE_BLOB = 3

	// Multiple context types and synchronization timelines
	// supported. Requires VIRTIO_GPU_F_VIRGL.
	VIRTIO_GPU_F_CONTEXT_INIT = 4
)

// Events.
const (
	VIRTIO_GPU_EVENT_DISPLAY = (1 << 0)
)

type GPU struct {
	MMIO
	Width  int
	Height int

	m          sync.Mutex
	window     *glfw.Window
	pixels     *image.RGBA
	frameDirty bool
	source     *GPUResource

	resources     map[uint32]*GPUResource
	controlEvents int
	cursorEvents  int
}

type GPUResource struct {
	ID     uint32
	Format GPUFormat
	Width  uint32
	Height uint32
	Pages  [][]byte
}

//go:generate stringer -type=GPUFormat

type GPUFormat uint32

const (
	VIRTIO_GPU_FORMAT_B8G8R8A8_UNORM GPUFormat = 1
	VIRTIO_GPU_FORMAT_B8G8R8X8_UNORM GPUFormat = 2
	VIRTIO_GPU_FORMAT_A8R8G8B8_UNORM GPUFormat = 3
	VIRTIO_GPU_FORMAT_X8R8G8B8_UNORM GPUFormat = 4

	VIRTIO_GPU_FORMAT_R8G8B8A8_UNORM GPUFormat = 67
	VIRTIO_GPU_FORMAT_X8B8G8R8_UNORM GPUFormat = 68

	VIRTIO_GPU_FORMAT_A8B8G8R8_UNORM GPUFormat = 121
	VIRTIO_GPU_FORMAT_R8G8B8X8_UNORM GPUFormat = 134
)

func NewGPU(hart isa.Hart, start uint64, plic *dev.PLIC, irq uint32,
	mem *memory.Memory, width, height int) (*GPU, error) {

	vio := &GPU{
		MMIO: MMIO{
			Log: logger.Log{
				Name:  "virtio-gpu",
				Level: logger.Info,
			},
			DeviceID: GPUDeviceID,
			Features: 0,
			Hart:     hart,
			Start:    start,
			End:      start + GPUSize,
			Plic:     plic,
			IRQ:      irq,
			Mem:      mem,
		},
		Width:  width,
		Height: height,
		pixels: image.NewRGBA(image.Rectangle{
			Max: image.Point{width, height},
		}),
		resources: make(map[uint32]*GPUResource),
	}
	vio.Init(2)
	vio.MMIO.Handler = vio

	vio.Infof("screen: %v\u00d7%v", vio.Width, vio.Height)

	return vio, nil
}

func (vio *GPU) EventLoop() {
	vio.Debugf("eventLoop starting")

	img := loadImage("../../docs/goemu-small.png")
	vio.drawImage(img)
	vio.frameDirty = true

	err := glfw.Init()
	if err != nil {
		vio.Errorf("glfw.Init: %v", err)
		return
	}

	vio.window, err = glfw.CreateWindow(vio.Width, vio.Height, "GoEMU",
		nil, nil)
	if err != nil {
		vio.Errorf("glfw.CreateWindow: %v", err)
		return
	}
	vio.window.MakeContextCurrent()
	err = gl.Init()
	if err != nil {
		vio.Errorf("window.MakeContextCurrent: %v", err)
		vio.window.Destroy()
		return
	}

	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)

	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.RGBA,
		int32(vio.Width),
		int32(vio.Height),
		0,
		gl.RGBA,
		gl.UNSIGNED_BYTE,
		gl.Ptr(vio.pixels.Pix),
	)

	for !vio.window.ShouldClose() {
		if vio.frameDirty {
			vio.frameDirty = false
			vio.m.Lock()
			gl.ClearColor(1, 1, 1, 1)
			gl.Clear(gl.COLOR_BUFFER_BIT)

			gl.Enable(gl.TEXTURE_2D)
			gl.BindTexture(gl.TEXTURE_2D, tex)

			gl.TexSubImage2D(
				gl.TEXTURE_2D,
				0,
				0,
				0,
				int32(vio.Width),
				int32(vio.Height),
				gl.RGBA,
				gl.UNSIGNED_BYTE,
				gl.Ptr(vio.pixels.Pix),
			)

			gl.Begin(gl.QUADS)

			gl.TexCoord2f(0, 1)
			gl.Vertex2f(-1, -1)

			gl.TexCoord2f(1, 1)
			gl.Vertex2f(1, -1)

			gl.TexCoord2f(1, 0)
			gl.Vertex2f(1, 1)

			gl.TexCoord2f(0, 0)
			gl.Vertex2f(-1, 1)

			gl.End()
			vio.m.Unlock()

			vio.window.SwapBuffers()
		}
		glfw.PollEvents()
	}

	vio.Debugf("eventLoop terminating")
	glfw.Terminate()
}

func loadImage(filename string) image.Image {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		log.Fatal(err)
	}

	return img
}

func (vio *GPU) drawImage(img image.Image) {
	b := img.Bounds()
	screenbb := vio.pixels.Bounds()

	// Clear screen.
	draw.Draw(vio.pixels, screenbb, image.White, image.Point{}, draw.Src)

	x := (screenbb.Dx() - b.Dx()) / 2
	y := (screenbb.Dy() - b.Dy()) / 2
	dstRect := image.Rect(x, y, x+b.Dx(), y+b.Dy())

	draw.Draw(vio.pixels, dstRect, img, b.Min, draw.Over)
}

// Reset implements Handler.Reset.
func (vio *GPU) Reset() error {
	// glfw.Terminate()
	return nil
}

// DeviceStats implements Handler.DeviceStats
func (vio *GPU) DeviceStats() {
	fmt.Printf("%v: control: %v\n", vio.Name, vio.controlEvents)
	fmt.Printf("%v: cursor : %v\n", vio.Name, vio.cursorEvents)
}

// ExecuteDescriptorChain implements Handler.ExecuteDescriptorChain.
func (vio *GPU) ExecuteDescriptorChain(vq *Queue, idx uint16) (uint32, error) {
	vio.Debugf("vq%v: chain: idx=%v", vq.Index, idx)

	desc, err := vq.loadDesc(idx)
	if err != nil {
		return 0, err
	}
	vio.Debugf("desc: %v", desc)

	if desc.Len < 24 {
		return 0, fmt.Errorf("truncated virtio_gpu_ctrl_hdr: %v", desc.Len)
	}
	chunk, err := vio.guestData(desc.Addr, uint64(desc.Len))
	if err != nil {
		return 0, err
	}
	vio.Tracef("virtio_gpu_ctrl_hdr:\n%s", hex.Dump(chunk))

	hdr := vio.decodeHdr(chunk)
	vio.Debugf("hdr: %v", hdr)

	var bufs [][]byte
	var writable []bool

	for desc.Flags&VIRTQ_DESC_F_NEXT != 0 {
		desc, err = vq.loadDesc(desc.Next)
		if err != nil {
			return 0, err
		}
		vio.Debugf("desc: %v", desc)
		buf, err := vio.guestData(desc.Addr, uint64(desc.Len))
		if err != nil {
			return 0, err
		}
		bufs = append(bufs, buf)
		writable = append(writable, desc.Flags&VIRTQ_DESC_F_WRITE != 0)
	}
	var transferred uint32
	switch hdr.Type {
	case VIRTIO_GPU_CMD_GET_EDID:
		transferred, err = vio.cmdGetEDID(hdr, chunk, bufs, writable)
	case VIRTIO_GPU_CMD_GET_DISPLAY_INFO:
		transferred, err = vio.cmdGetDisplayInfo(hdr, chunk, bufs, writable)
	case VIRTIO_GPU_CMD_RESOURCE_CREATE_2D:
		transferred, err = vio.cmdResourceCreate2D(hdr, chunk, bufs, writable)
	case VIRTIO_GPU_CMD_RESOURCE_ATTACH_BACKING:
		transferred, err = vio.cmdResourceAttachBacking(hdr, chunk, bufs,
			writable)
	case VIRTIO_GPU_CMD_SET_SCANOUT:
		transferred, err = vio.cmdSetScanout(hdr, chunk, bufs, writable)
	case VIRTIO_GPU_CMD_TRANSFER_TO_HOST_2D:
		transferred, err = vio.cmdTransferToHost2D(hdr, chunk, bufs, writable)
	case VIRTIO_GPU_CMD_RESOURCE_FLUSH:
		transferred, err = vio.cmdResourceFlush(hdr, chunk, bufs, writable)
	case VIRTIO_GPU_CMD_RESOURCE_UNREF:
		transferred, err = vio.cmdResourceUnref(hdr, chunk, bufs, writable)
	default:
		err = fmt.Errorf("%v: not implemented yet", hdr.Type)
	}
	if err != nil {
		return 0, err
	}

	return transferred, nil
}

var gpuRegs = map[uint64]string{
	0x100: "EventsRead",
	0x104: "EventsClear",
	0x108: "NumScanouts",
	0x10c: "NumCapsets",
}

func (vio *GPU) Load32(paddr uint64) (uint32, error) {
	offset := paddr - vio.Start

	reg, ok := gpuRegs[offset]
	if ok {
		vio.Debugf("Load32(%v[0x%03x])", reg, offset)
	}

	switch offset {
	// 5.7.4 Device configuration layout at offset 0x100.
	case 0x100: // EventsRead
		return 0, nil

	case 0x104: // EventsClear
		return 0, nil

	case 0x108: // NumScanouts
		return 1, nil

	case 0x10c: // NumCapsets
		return 0, nil

	default:
		return vio.MMIO.Load32(paddr)
	}
}

func (vio *GPU) Store32(paddr uint64, v uint32) error {
	offset := paddr - vio.Start

	reg, ok := gpuRegs[offset]
	if ok {
		vio.Debugf("Store32(%v[0x%03x], %v)", reg, offset, v)
	}

	switch offset {
	case 0x100, 0x108, 0x10c:
		return fmt.Errorf("read-only register 0x%03x", offset)

	case 0x104:
		return nil

	default:
		return vio.MMIO.Store32(paddr, v)
	}
}

//go:generate stringer -type=GPUCtrlType

type GPUCtrlType uint32

/* 2d commands */
const (
	VIRTIO_GPU_CMD_GET_DISPLAY_INFO GPUCtrlType = iota + 0x0100
	VIRTIO_GPU_CMD_RESOURCE_CREATE_2D
	VIRTIO_GPU_CMD_RESOURCE_UNREF
	VIRTIO_GPU_CMD_SET_SCANOUT
	VIRTIO_GPU_CMD_RESOURCE_FLUSH
	VIRTIO_GPU_CMD_TRANSFER_TO_HOST_2D
	VIRTIO_GPU_CMD_RESOURCE_ATTACH_BACKING
	VIRTIO_GPU_CMD_RESOURCE_DETACH_BACKING
	VIRTIO_GPU_CMD_GET_CAPSET_INFO
	VIRTIO_GPU_CMD_GET_CAPSET
	VIRTIO_GPU_CMD_GET_EDID
	VIRTIO_GPU_CMD_RESOURCE_ASSIGN_UUID
	VIRTIO_GPU_CMD_RESOURCE_CREATE_BLOB
	VIRTIO_GPU_CMD_SET_SCANOUT_BLOB
)

/* 3d commands */
const (
	VIRTIO_GPU_CMD_CTX_CREATE GPUCtrlType = iota + 0x0200
	VIRTIO_GPU_CMD_CTX_DESTROY
	VIRTIO_GPU_CMD_CTX_ATTACH_RESOURCE
	VIRTIO_GPU_CMD_CTX_DETACH_RESOURCE
	VIRTIO_GPU_CMD_RESOURCE_CREATE_3D
	VIRTIO_GPU_CMD_TRANSFER_TO_HOST_3D
	VIRTIO_GPU_CMD_TRANSFER_FROM_HOST_3D
	VIRTIO_GPU_CMD_SUBMIT_3D
	VIRTIO_GPU_CMD_RESOURCE_MAP_BLOB
	VIRTIO_GPU_CMD_RESOURCE_UNMAP_BLOB
)

/* cursor commands */
const (
	VIRTIO_GPU_CMD_UPDATE_CURSOR GPUCtrlType = iota + 0x0300
	VIRTIO_GPU_CMD_MOVE_CURSOR
)

/* success responses */
const (
	VIRTIO_GPU_RESP_OK_NODATA uint32 = iota + 0x1100
	VIRTIO_GPU_RESP_OK_DISPLAY_INFO
	VIRTIO_GPU_RESP_OK_CAPSET_INFO
	VIRTIO_GPU_RESP_OK_CAPSET
	VIRTIO_GPU_RESP_OK_EDID
	VIRTIO_GPU_RESP_OK_RESOURCE_UUID
	VIRTIO_GPU_RESP_OK_MAP_INFO
)

/* error responses */
const (
	VIRTIO_GPU_RESP_ERR_UNSPEC uint32 = iota + 0x1200
	VIRTIO_GPU_RESP_ERR_OUT_OF_MEMORY
	VIRTIO_GPU_RESP_ERR_INVALID_SCANOUT_ID
	VIRTIO_GPU_RESP_ERR_INVALID_RESOURCE_ID
	VIRTIO_GPU_RESP_ERR_INVALID_CONTEXT_ID
	VIRTIO_GPU_RESP_ERR_INVALID_PARAMETER
)

type GPUCtrlHdr struct {
	Type    GPUCtrlType
	Flags   uint32
	FenceID uint64
	CtxID   uint32
	RingIdx uint8
	Pad1    uint8
	Pad2    uint8
	Pad3    uint8
}

func (hdr *GPUCtrlHdr) String() string {
	return fmt.Sprintf("%v: flags=%x, fenceID=%v, ctxID=%v, ringIdx=%v",
		hdr.Type, hdr.Flags, hdr.FenceID, hdr.CtxID, hdr.RingIdx)
}

func (vio *GPU) decodeHdr(data []byte) *GPUCtrlHdr {
	return &GPUCtrlHdr{
		Type:    GPUCtrlType(vioBO.Uint32(data[0:])),
		Flags:   vioBO.Uint32(data[4:]),
		FenceID: vioBO.Uint64(data[8:]),
		CtxID:   vioBO.Uint32(data[16:]),
		RingIdx: data[20],
	}
}

func (vio *GPU) cmdGetEDID(hdr *GPUCtrlHdr, hdrBuf []byte, bufs [][]byte,
	writable []bool) (uint32, error) {
	if len(bufs) != 1 || !writable[0] {
		return 0, fmt.Errorf("%v: invalid output buffers", hdr.Type)
	}

	return 0, fmt.Errorf("VIRTIO_GPU_CMD_GET_EDID not implemented yet")
}

type GPURect struct {
	X      uint32
	Y      uint32
	Width  uint32
	Height uint32
}

func (r *GPURect) String() string {
	return fmt.Sprintf("x=%v,y=%v,w=%v,h=%v", r.X, r.Y, r.Width, r.Height)
}

type GPUDisplayOne struct {
	GPURect
	Enabled uint32
	Flags   uint32
}

func (vio *GPU) cmdGetDisplayInfo(hdr *GPUCtrlHdr, hdrBuf []byte, bufs [][]byte,
	writable []bool) (uint32, error) {
	if len(bufs) != 1 || !writable[0] {
		return 0, fmt.Errorf("%v: invalid output buffers", hdr.Type)
	}
	buf := bufs[0]
	vioBO.PutUint32(buf[0:], VIRTIO_GPU_RESP_OK_DISPLAY_INFO)
	// struct virtio_gpu_display_one
	vioBO.PutUint32(buf[24:], 0) // x
	vioBO.PutUint32(buf[28:], 0) // y
	vioBO.PutUint32(buf[32:], uint32(vio.Width))
	vioBO.PutUint32(buf[36:], uint32(vio.Height))
	vioBO.PutUint32(buf[40:], 1) // enabled
	vioBO.PutUint32(buf[44:], 0) // flags

	return 48, nil
}

type GPUResourceCreate2D struct {
	GPUCtrlHdr
	ResourceID uint32
	Format     GPUFormat
	Width      uint32
	Height     uint32
}

func (vio *GPU) cmdResourceCreate2D(hdr *GPUCtrlHdr, hdrBuf []byte,
	bufs [][]byte, writable []bool) (uint32, error) {
	if len(bufs) != 1 || !writable[0] {
		return 0, fmt.Errorf("%v: invalid output buffers", hdr.Type)
	}
	if len(hdrBuf) < 40 {
		return 0, fmt.Errorf("%v: truncated request", hdr.Type)
	}

	resource := &GPUResource{
		ID:     vioBO.Uint32(hdrBuf[24:]),
		Format: GPUFormat(vioBO.Uint32(hdrBuf[28:])),
		Width:  vioBO.Uint32(hdrBuf[32:]),
		Height: vioBO.Uint32(hdrBuf[36:]),
	}
	// XXX format

	vio.Debugf("%v: resourceID=%v, format=%v, width=%v, height=%v",
		hdr.Type, resource.ID, resource.Format, resource.Width,
		resource.Height)
	vio.resources[resource.ID] = resource

	buf := bufs[0]
	vioBO.PutUint32(buf[0:], VIRTIO_GPU_RESP_OK_NODATA)

	return uint32(len(hdrBuf)) + 24, nil
}

func (vio *GPU) cmdResourceAttachBacking(hdr *GPUCtrlHdr, hdrBuf []byte,
	bufs [][]byte, writable []bool) (uint32, error) {
	if len(bufs) != 2 || writable[0] || !writable[1] {
		return 0, fmt.Errorf("%v: invalid buffers", hdr.Type)
	}
	if len(hdrBuf) < 32 {
		return 0, fmt.Errorf("%v: truncated request", hdr.Type)
	}
	resourceID := vioBO.Uint32(hdrBuf[24:])
	nrEntries := vioBO.Uint32(hdrBuf[28:])

	vio.Debugf("attach backing: resource=%v, #entries=%v",
		resourceID, nrEntries)

	resource, ok := vio.resources[resourceID]
	if !ok {
		vio.Errorf("unknown resource %v", resourceID)
		return 0, fmt.Errorf("unknown resource %v", resourceID)
	}
	// XXX this could be scattered across multiple chunks.
	input := bufs[0]
	for i := uint32(0); i < nrEntries; i++ {
		if len(input) < 16 {
			return 0, fmt.Errorf("truncated buffer")
		}
		addr := vioBO.Uint64(input[0:])
		length := vioBO.Uint32(input[8:])
		input = input[16:]

		buf, err := vio.guestData(addr, uint64(length))
		if err != nil {
			return 0, err
		}
		resource.Pages = append(resource.Pages, buf)
	}
	vioBO.PutUint32(bufs[1][0:], VIRTIO_GPU_RESP_OK_NODATA)

	return uint32(len(hdrBuf) + len(bufs[0]) + len(bufs[1])), nil
}

func (vio *GPU) cmdSetScanout(hdr *GPUCtrlHdr, hdrBuf []byte,
	bufs [][]byte, writable []bool) (uint32, error) {
	if len(bufs) != 1 || !writable[0] {
		return 0, fmt.Errorf("%v: invalid output buffers", hdr.Type)
	}
	if len(hdrBuf) < 48 {
		return 0, fmt.Errorf("%v: truncated request", hdr.Type)
	}
	rect := GPURect{
		X:      vioBO.Uint32(hdrBuf[24:]),
		Y:      vioBO.Uint32(hdrBuf[28:]),
		Width:  vioBO.Uint32(hdrBuf[32:]),
		Height: vioBO.Uint32(hdrBuf[36:]),
	}
	scanoutID := vioBO.Uint32(hdrBuf[40:])
	resourceID := vioBO.Uint32(hdrBuf[44:])
	vio.Debugf("setScanout rect=%v, scanoutID=%v, resourceID=%v",
		rect, scanoutID, resourceID)
	if resourceID == 0 {
		vio.Debugf("disabling scanout")
		vio.source = nil
	} else {
		resource, ok := vio.resources[resourceID]
		if !ok {
			vio.Errorf("unknown resource %v", resourceID)
			return 0, fmt.Errorf("unknown resource %v", resourceID)
		}
		vio.Debugf("enabling scanout %v", scanoutID)
		vio.source = resource
	}

	vioBO.PutUint32(bufs[0][0:], VIRTIO_GPU_RESP_OK_NODATA)

	return uint32(len(hdrBuf) + len(bufs[0])), nil
}

func (vio *GPU) cmdTransferToHost2D(hdr *GPUCtrlHdr, hdrBuf []byte,
	bufs [][]byte, writable []bool) (uint32, error) {
	if len(bufs) != 1 || !writable[0] {
		return 0, fmt.Errorf("%v: invalid output buffers", hdr.Type)
	}
	if len(hdrBuf) < 56 {
		return 0, fmt.Errorf("%v: truncated request", hdr.Type)
	}
	rect := GPURect{
		X:      vioBO.Uint32(hdrBuf[24:]),
		Y:      vioBO.Uint32(hdrBuf[28:]),
		Width:  vioBO.Uint32(hdrBuf[32:]),
		Height: vioBO.Uint32(hdrBuf[36:]),
	}
	offset := vioBO.Uint64(hdrBuf[40:])
	resourceID := vioBO.Uint32(hdrBuf[48:])
	vio.Debugf("transferToHost2D rect=%v, offset=%v, resourceID=%v",
		rect, offset, resourceID)

	resource, ok := vio.resources[resourceID]
	if !ok {
		vio.Errorf("unknown resource %v", resourceID)
		return 0, fmt.Errorf("unknown resource %v", resourceID)
	}
	vio.m.Lock()
	defer vio.m.Unlock()

	var pageIndex, pageStart uint64

	stride := resource.Width * 4
	for localY := uint32(0); localY < rect.Height; localY++ {
		currentY := rect.Y + localY

		srcOfs := offset + uint64(currentY*stride+rect.X*4)
		dstOfs := uint64(currentY*stride + rect.X*4)
		count := uint64(rect.Width * 4)

		for i := uint64(0); i < count; i += 4 {
			page := resource.Pages[pageIndex]

			for pageStart+uint64(len(page)) <= srcOfs {
				pageStart += uint64(len(page))
				pageIndex++
			}
			pageOfs := srcOfs - pageStart

			vio.pixels.Pix[dstOfs+i+0] = page[pageOfs+i+2]
			vio.pixels.Pix[dstOfs+i+1] = page[pageOfs+i+1]
			vio.pixels.Pix[dstOfs+i+2] = page[pageOfs+i+0]
			vio.pixels.Pix[dstOfs+i+3] = 0xff
		}
	}

	vioBO.PutUint32(bufs[0][0:], VIRTIO_GPU_RESP_OK_NODATA)

	return uint32(len(hdrBuf) + len(bufs[0])), nil
}

func (vio *GPU) cmdResourceFlush(hdr *GPUCtrlHdr, hdrBuf []byte,
	bufs [][]byte, writable []bool) (uint32, error) {
	if len(bufs) != 1 || !writable[0] {
		return 0, fmt.Errorf("%v: invalid output buffers", hdr.Type)
	}
	if len(hdrBuf) < 48 {
		return 0, fmt.Errorf("%v: truncated request", hdr.Type)
	}
	rect := GPURect{
		X:      vioBO.Uint32(hdrBuf[24:]),
		Y:      vioBO.Uint32(hdrBuf[28:]),
		Width:  vioBO.Uint32(hdrBuf[32:]),
		Height: vioBO.Uint32(hdrBuf[36:]),
	}
	resourceID := vioBO.Uint32(hdrBuf[40:])
	vio.Debugf("cmdResourceFlush rect=%v, resourceID=%v",
		rect, resourceID)

	resource, ok := vio.resources[resourceID]
	if !ok {
		vio.Errorf("unknown resource %v", resourceID)
		return 0, fmt.Errorf("unknown resource %v", resourceID)
	}
	_ = resource

	vio.frameDirty = true
	glfw.PostEmptyEvent()

	vioBO.PutUint32(bufs[0][0:], VIRTIO_GPU_RESP_OK_NODATA)

	return uint32(len(hdrBuf) + len(bufs[0])), nil
}

func (vio *GPU) cmdResourceUnref(hdr *GPUCtrlHdr, hdrBuf []byte,
	bufs [][]byte, writable []bool) (uint32, error) {
	if len(bufs) != 1 || !writable[0] {
		return 0, fmt.Errorf("%v: invalid output buffers", hdr.Type)
	}
	if len(hdrBuf) < 32 {
		return 0, fmt.Errorf("%v: truncated request", hdr.Type)
	}
	resourceID := vioBO.Uint32(hdrBuf[24:])
	_, ok := vio.resources[resourceID]
	if !ok {
		vio.Errorf("unknown resource %v", resourceID)
		return 0, fmt.Errorf("unknown resource %v", resourceID)
	}
	delete(vio.resources, resourceID)

	vioBO.PutUint32(bufs[0][0:], VIRTIO_GPU_RESP_OK_NODATA)

	return uint32(len(hdrBuf) + len(bufs[0])), nil
}

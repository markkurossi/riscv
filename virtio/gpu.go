//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package virtio

//lint:file-ignore ST1003 to match the C coding style for constants.

// XXX virtio-gpu: ERROR: execute descriptor chain: VIRTIO_GPU_CMD_UPDATE_CURSOR: not implemented yet

import (
	"encoding/hex"
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
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
	Title  string
	Width  int
	Height int

	InputListener InputListener
	lastX         int
	lastY         int

	renderM     sync.Mutex
	pending     bool
	pendingDone bool
	window      *glfw.Window
	pixels      *image.RGBA
	frameDirty  bool
	source      *GPUResource

	transferC chan transferReq

	resources map[uint32]*GPUResource
	logo      image.Image
	events    [2]int
}

type transferReq struct {
	index    uint32
	rect     GPURect
	offset   uint64
	resource *GPUResource
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
	mem *memory.Memory, title string, width, height int) (*GPU, error) {

	logo, err := loadImage("../../docs/goemu-small.png")
	if err != nil {
		return nil, err
	}

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
		Title:  title,
		Width:  width,
		Height: height,
		pixels: image.NewRGBA(image.Rectangle{
			Max: image.Point{width, height},
		}),
		transferC: make(chan transferReq, 1),
		resources: make(map[uint32]*GPUResource),
		logo:      logo,
	}
	vio.Init(2)
	vio.MMIO.Handler = vio

	vio.Infof("screen: %v\u00d7%v", vio.Width, vio.Height)
	go vio.converter()

	return vio, nil
}

func (vio *GPU) EventLoop() {
	vio.Debugf("eventLoop starting")

	vio.drawImage(vio.logo)
	vio.frameDirty = true

	err := glfw.Init()
	if err != nil {
		vio.Errorf("glfw.Init: %v", err)
		return
	}

	title := "GoEMU"
	if len(vio.Title) != 0 {
		title += " \u2014 " + vio.Title
	}

	vio.window, err = glfw.CreateWindow(vio.Width, vio.Height, title, nil, nil)
	if err != nil {
		vio.Errorf("glfw.CreateWindow: %v", err)
		return
	}
	vio.window.SetCursorPosCallback(vio.cursorPosCallback)
	vio.window.SetMouseButtonCallback(vio.mouseButtonCallback)
	vio.window.SetKeyCallback(vio.keyCallback)

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
			vio.renderM.Lock()
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
			vio.renderM.Unlock()

			vio.window.SwapBuffers()
		}
		glfw.WaitEvents()
	}

	vio.Debugf("eventLoop terminating")
	glfw.Terminate()
}

func loadImage(filename string) (image.Image, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}

	return img, nil
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

// Halt implements mmu.MMIO.Halt.
func (vio *GPU) Halt() error {
	return vio.Reset()
}

// Reset implements Handler.Reset.
func (vio *GPU) Reset() error {
	vio.drawImage(vio.logo)
	vio.frameDirty = true
	glfw.PostEmptyEvent()

	return nil
}

// DeviceStats implements Handler.DeviceStats
func (vio *GPU) DeviceStats() {
	fmt.Printf("%v: control : %v\n", vio.Name, vio.events[0])
	fmt.Printf("%v: cursor  : %v\n", vio.Name, vio.events[1])
}

// ExecuteDescriptorChain implements Handler.ExecuteDescriptorChain.
func (vio *GPU) ExecuteDescriptorChain(vq *Queue, idx uint16) (uint32, error) {
	vio.Debugf("vq%v: chain: idx=%v", vq.Index, idx)

	if vq.Index == 0 && vio.pending {
		return 0, nil
	}
	vio.events[vq.Index]++

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
		transferred, err = vio.cmdTransferToHost2D(vq, hdr, chunk, bufs,
			writable)
	case VIRTIO_GPU_CMD_RESOURCE_FLUSH:
		transferred, err = vio.cmdResourceFlush(hdr, chunk, bufs, writable)
	case VIRTIO_GPU_CMD_RESOURCE_UNREF:
		transferred, err = vio.cmdResourceUnref(hdr, chunk, bufs, writable)

	case VIRTIO_GPU_CMD_UPDATE_CURSOR:
		transferred, err = vio.cmdUpdateCursor(hdr, chunk, bufs, writable)

	case VIRTIO_GPU_CMD_MOVE_CURSOR:
		transferred, err = vio.cmdMoveCursor(hdr, chunk, bufs, writable)

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
	VIRTIO_GPU_RESP_OK_NODATA GPUCtrlType = iota + 0x1100
	VIRTIO_GPU_RESP_OK_DISPLAY_INFO
	VIRTIO_GPU_RESP_OK_CAPSET_INFO
	VIRTIO_GPU_RESP_OK_CAPSET
	VIRTIO_GPU_RESP_OK_EDID
	VIRTIO_GPU_RESP_OK_RESOURCE_UUID
	VIRTIO_GPU_RESP_OK_MAP_INFO
)

/* error responses */
const (
	VIRTIO_GPU_RESP_ERR_UNSPEC GPUCtrlType = iota + 0x1200
	VIRTIO_GPU_RESP_ERR_OUT_OF_MEMORY
	VIRTIO_GPU_RESP_ERR_INVALID_SCANOUT_ID
	VIRTIO_GPU_RESP_ERR_INVALID_RESOURCE_ID
	VIRTIO_GPU_RESP_ERR_INVALID_CONTEXT_ID
	VIRTIO_GPU_RESP_ERR_INVALID_PARAMETER
)

// Control header flags.
const (
	VIRTIO_GPU_FLAG_FENCE uint32 = 1 << iota
	VIRTIO_GPU_FLAG_INFO_RING_IDX
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

func (hdr *GPUCtrlHdr) Response(code GPUCtrlType, buf []byte) {
	if len(buf) < 24 {
		return
	}
	vioBO.PutUint32(buf[0:], uint32(code))

	if hdr.Flags&VIRTIO_GPU_FLAG_FENCE != 0 {
		vioBO.PutUint32(buf[4:], VIRTIO_GPU_FLAG_FENCE)
		vioBO.PutUint64(buf[8:], hdr.FenceID)
	}
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

	hdr.Response(VIRTIO_GPU_RESP_OK_DISPLAY_INFO, buf[0:])

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

	vio.Infof("%v: resourceID=%v, format=%v, width=%v, height=%v",
		hdr.Type, resource.ID, resource.Format, resource.Width,
		resource.Height)
	vio.resources[resource.ID] = resource

	buf := bufs[0]
	hdr.Response(VIRTIO_GPU_RESP_OK_NODATA, buf)

	return 24, nil
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
	hdr.Response(VIRTIO_GPU_RESP_OK_NODATA, bufs[1][0:])

	return 24, nil
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

	hdr.Response(VIRTIO_GPU_RESP_OK_NODATA, bufs[0][0:])

	return 24, nil
}

func (vio *GPU) cmdTransferToHost2D(vq *Queue, hdr *GPUCtrlHdr, hdrBuf []byte,
	bufs [][]byte, writable []bool) (uint32, error) {
	if len(bufs) != 1 || !writable[0] {
		return 0, fmt.Errorf("%v: invalid output buffers", hdr.Type)
	}
	if len(hdrBuf) < 56 {
		return 0, fmt.Errorf("%v: truncated request", hdr.Type)
	}
	if vio.pendingDone {
		vio.pendingDone = false
		vio.Debugf("completing pending cmdTransferToHost2D")
		hdr.Response(VIRTIO_GPU_RESP_OK_NODATA, bufs[0][0:])
		return 24, nil
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
	vio.Debugf("queuing transferReq...")
	vio.transferC <- transferReq{
		index:    vq.Index,
		rect:     rect,
		offset:   offset,
		resource: resource,
	}
	vio.pending = true
	vio.Debugf("queuing transferReq done")
	return 0, nil
}

func (vio *GPU) converter() {
	for {
		req := <-vio.transferC
		vio.Debugf("converter: rect: %v", req.rect)
		vio.renderM.Lock()

		var pageIndex, pageStart uint64

		stride := req.resource.Width * 4
		for localY := uint32(0); localY < req.rect.Height; localY++ {
			currentY := req.rect.Y + localY

			srcOfs := req.offset + uint64(currentY*stride+req.rect.X*4)
			dstOfs := uint64(currentY*stride + req.rect.X*4)
			count := uint64(req.rect.Width * 4)

			for i := uint64(0); i < count; i += 4 {
				page := req.resource.Pages[pageIndex]

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
		vio.renderM.Unlock()

		vio.M.Lock()
		vio.pending = false
		vio.pendingDone = true
		vio.Debugf("converter: ProcessQueue(%v)...", req.index)
		vio.ProcessQueue(req.index)
		vio.Debugf("converter: ProcessQueue(%v) done", req.index)
		vio.M.Unlock()
	}
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

	hdr.Response(VIRTIO_GPU_RESP_OK_NODATA, bufs[0][0:])

	return 24, nil
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

	hdr.Response(VIRTIO_GPU_RESP_OK_NODATA, bufs[0][0:])

	return 24, nil
}

func (vio *GPU) cmdUpdateCursor(hdr *GPUCtrlHdr, hdrBuf []byte,
	bufs [][]byte, writable []bool) (uint32, error) {

	if len(bufs) > 0 && writable[0] {
		hdr.Response(VIRTIO_GPU_RESP_OK_NODATA, bufs[0])
	}

	return 24, nil
}

func (vio *GPU) cmdMoveCursor(hdr *GPUCtrlHdr, hdrBuf []byte,
	bufs [][]byte, writable []bool) (uint32, error) {

	if len(bufs) > 0 && writable[0] {
		hdr.Response(VIRTIO_GPU_RESP_OK_NODATA, bufs[0])
	}

	return 24, nil
}

func (vio *GPU) cursorPosCallback(w *glfw.Window, x, y float64) {
	ix := int(x)
	iy := int(y)

	if ix == vio.lastX && iy == vio.lastY {
		return
	}
	vio.lastX = ix
	vio.lastY = iy

	vio.Tracef("cursor: x=%v, y=%v", ix, iy)

	if vio.InputListener == nil ||
		ix < 0 || ix >= vio.Width || iy < 0 || iy >= vio.Height {
		return
	}

	vio.InputListener.OnMouseMove(int32(ix), int32(iy))
}

func (vio *GPU) mouseButtonCallback(w *glfw.Window, button glfw.MouseButton,
	action glfw.Action, mod glfw.ModifierKey) {
	vio.Tracef("button: button=%v, action=%v, mod=%v", button, action, mod)

	k, ok := mouseButtonMap[button]
	if !ok {
		vio.Tracef("skipping: button=%v, action=%v, mod=%v",
			button, action, mod)
		return
	}
	if vio.InputListener != nil {
		switch action {
		case glfw.Release:
			vio.InputListener.OnButtonRelease(k)
		case glfw.Press:
			vio.InputListener.OnButtonPress(k)
		case glfw.Repeat:
			vio.InputListener.OnButtonRepeat(k)
		}
	}
}

func (vio *GPU) keyCallback(w *glfw.Window, key glfw.Key, scancode int,
	action glfw.Action, mod glfw.ModifierKey) {
	vio.Tracef("key: key=%v, scancode=%v, action=%v, mod=%v",
		key, scancode, action, mod)

	k, ok := inputKeyMap[key]
	if !ok {
		vio.Tracef("skipping: key=%v, scancode=%v, action=%v, mod=%v",
			key, scancode, action, mod)
		return
	}
	if vio.InputListener != nil {
		switch action {
		case glfw.Release:
			vio.InputListener.OnKeyRelease(k)
		case glfw.Press:
			vio.InputListener.OnKeyPress(k)
		case glfw.Repeat:
			vio.InputListener.OnKeyRepeat(k)
		}
	}
}

var inputKeyMap = map[glfw.Key]Key{
	glfw.KeyUnknown:      KEY_UNKNOWN,
	glfw.KeySpace:        KEY_SPACE,
	glfw.KeyApostrophe:   KEY_APOSTROPHE,
	glfw.KeyComma:        KEY_COMMA,
	glfw.KeyMinus:        KEY_MINUS,
	glfw.KeyPeriod:       KEY_DOT,
	glfw.KeySlash:        KEY_SLASH,
	glfw.Key0:            KEY_0,
	glfw.Key1:            KEY_1,
	glfw.Key2:            KEY_2,
	glfw.Key3:            KEY_3,
	glfw.Key4:            KEY_4,
	glfw.Key5:            KEY_5,
	glfw.Key6:            KEY_6,
	glfw.Key7:            KEY_7,
	glfw.Key8:            KEY_8,
	glfw.Key9:            KEY_9,
	glfw.KeySemicolon:    KEY_SEMICOLON,
	glfw.KeyEqual:        KEY_EQUAL,
	glfw.KeyA:            KEY_A,
	glfw.KeyB:            KEY_B,
	glfw.KeyC:            KEY_C,
	glfw.KeyD:            KEY_D,
	glfw.KeyE:            KEY_E,
	glfw.KeyF:            KEY_F,
	glfw.KeyG:            KEY_G,
	glfw.KeyH:            KEY_H,
	glfw.KeyI:            KEY_I,
	glfw.KeyJ:            KEY_J,
	glfw.KeyK:            KEY_K,
	glfw.KeyL:            KEY_L,
	glfw.KeyM:            KEY_M,
	glfw.KeyN:            KEY_N,
	glfw.KeyO:            KEY_O,
	glfw.KeyP:            KEY_P,
	glfw.KeyQ:            KEY_Q,
	glfw.KeyR:            KEY_R,
	glfw.KeyS:            KEY_S,
	glfw.KeyT:            KEY_T,
	glfw.KeyU:            KEY_U,
	glfw.KeyV:            KEY_V,
	glfw.KeyW:            KEY_W,
	glfw.KeyX:            KEY_X,
	glfw.KeyY:            KEY_Y,
	glfw.KeyZ:            KEY_Z,
	glfw.KeyLeftBracket:  KEY_LEFTBRACE,
	glfw.KeyBackslash:    KEY_BACKSLASH,
	glfw.KeyRightBracket: KEY_RIGHTBRACE,
	glfw.KeyGraveAccent:  KEY_GRAVE,
	glfw.KeyWorld1:       KEY_UNKNOWN,
	glfw.KeyWorld2:       KEY_UNKNOWN,
	glfw.KeyEscape:       KEY_ESC,
	glfw.KeyEnter:        KEY_ENTER,
	glfw.KeyTab:          KEY_TAB,
	glfw.KeyBackspace:    KEY_BACKSPACE,
	glfw.KeyInsert:       KEY_INSERT,
	glfw.KeyDelete:       KEY_DELETE,
	glfw.KeyRight:        KEY_RIGHT,
	glfw.KeyLeft:         KEY_LEFT,
	glfw.KeyDown:         KEY_DOWN,
	glfw.KeyUp:           KEY_UP,
	glfw.KeyPageUp:       KEY_PAGEUP,
	glfw.KeyPageDown:     KEY_PAGEDOWN,
	glfw.KeyHome:         KEY_HOME,
	glfw.KeyEnd:          KEY_END,
	glfw.KeyCapsLock:     KEY_CAPSLOCK,
	glfw.KeyScrollLock:   KEY_SCROLLLOCK,
	glfw.KeyNumLock:      KEY_NUMLOCK,
	glfw.KeyPrintScreen:  KEY_PRINT,
	glfw.KeyPause:        KEY_PAUSE,
	glfw.KeyF1:           KEY_F1,
	glfw.KeyF2:           KEY_F2,
	glfw.KeyF3:           KEY_F3,
	glfw.KeyF4:           KEY_F4,
	glfw.KeyF5:           KEY_F5,
	glfw.KeyF6:           KEY_F6,
	glfw.KeyF7:           KEY_F7,
	glfw.KeyF8:           KEY_F8,
	glfw.KeyF9:           KEY_F9,
	glfw.KeyF10:          KEY_F10,
	glfw.KeyF11:          KEY_F11,
	glfw.KeyF12:          KEY_F12,
	glfw.KeyF13:          KEY_F13,
	glfw.KeyF14:          KEY_F14,
	glfw.KeyF15:          KEY_F15,
	glfw.KeyF16:          KEY_F16,
	glfw.KeyF17:          KEY_F17,
	glfw.KeyF18:          KEY_F18,
	glfw.KeyF19:          KEY_F19,
	glfw.KeyF20:          KEY_F20,
	glfw.KeyF21:          KEY_F21,
	glfw.KeyF22:          KEY_F22,
	glfw.KeyF23:          KEY_F23,
	glfw.KeyF24:          KEY_F24,
	glfw.KeyF25:          KEY_UNKNOWN,
	glfw.KeyKP0:          KEY_KP0,
	glfw.KeyKP1:          KEY_KP1,
	glfw.KeyKP2:          KEY_KP2,
	glfw.KeyKP3:          KEY_KP3,
	glfw.KeyKP4:          KEY_KP4,
	glfw.KeyKP5:          KEY_KP5,
	glfw.KeyKP6:          KEY_KP6,
	glfw.KeyKP7:          KEY_KP7,
	glfw.KeyKP8:          KEY_KP8,
	glfw.KeyKP9:          KEY_KP9,
	glfw.KeyKPDecimal:    KEY_KPDOT,
	glfw.KeyKPDivide:     KEY_KPSLASH,
	glfw.KeyKPMultiply:   KEY_KPASTERISK,
	glfw.KeyKPSubtract:   KEY_KPMINUS,
	glfw.KeyKPAdd:        KEY_KPPLUS,
	glfw.KeyKPEnter:      KEY_KPENTER,
	glfw.KeyKPEqual:      KEY_KPEQUAL,
	glfw.KeyLeftShift:    KEY_LEFTSHIFT,
	glfw.KeyLeftControl:  KEY_LEFTCTRL,
	glfw.KeyLeftAlt:      KEY_LEFTALT,
	glfw.KeyLeftSuper:    KEY_LEFTMETA,
	glfw.KeyRightShift:   KEY_RIGHTSHIFT,
	glfw.KeyRightControl: KEY_RIGHTCTRL,
	glfw.KeyRightAlt:     KEY_RIGHTALT,
	glfw.KeyRightSuper:   KEY_RIGHTMETA,
	glfw.KeyMenu:         KEY_MENU,
}

var mouseButtonMap = map[glfw.MouseButton]MouseButton{
	glfw.MouseButtonLeft:   BTN_LEFT,
	glfw.MouseButtonRight:  BTN_RIGHT,
	glfw.MouseButtonMiddle: BTN_MIDDLE,
}

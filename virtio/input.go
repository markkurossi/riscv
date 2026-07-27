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

	"github.com/markkurossi/riscv/dev"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/logger"
	"github.com/markkurossi/riscv/memory"
)

const (
	InputDeviceID = 18
	InputSize     = 4096
)

type Input struct {
	MMIO

	Width  int
	Height int
	absX   []byte
	absY   []byte

	sel    uint8
	subsel uint8
	size   uint8

	recvCh  chan uint16
	eventCh chan *InputEvent

	stats [2]int
}

type InputEvent struct {
	Type  uint16
	Code  uint16
	Value uint32
}

func NewInput(hart isa.Hart, start uint64, plic *dev.PLIC, irq uint32,
	mem *memory.Memory, width, height int) *Input {

	vio := &Input{
		MMIO: MMIO{
			Log: logger.Log{
				Name:  "virtio-input",
				Level: logger.Info,
			},
			DeviceID: InputDeviceID,
			Hart:     hart,
			Start:    start,
			End:      start + InputSize,
			Plic:     plic,
			IRQ:      irq,
			Mem:      mem,
		},
		Width:   width,
		Height:  height,
		recvCh:  make(chan uint16, queueNumMax),
		eventCh: make(chan *InputEvent, queueNumMax),
	}
	plic.IRQs[irq] = "input"

	vio.Init(2)
	vio.MMIO.Handler = vio

	// struct virtio_input_absinfo {
	//   le32  min;
	//   le32  max;
	//   le32  fuzz;
	//   le32  flat;
	//   le32  res;
	// };

	vio.Infof("screen: %v\u00d7%v", vio.Width, vio.Height)

	vio.absX = make([]byte, 20)
	vioBO.PutUint32(vio.absX[4:], uint32(vio.Width-1))

	vio.absY = make([]byte, 20)
	vioBO.PutUint32(vio.absY[4:], uint32(vio.Height-1))

	go vio.receiver(vio.queues[0])

	return vio
}

// Ready implements Handler.Ready.
func (vio *Input) Ready() {
}

// Reset implements Handler.Reset.
func (vio *Input) Reset() error {
	return nil
}

// DeviceStats implements Handler.DeviceStats
func (vio *Input) DeviceStats() {
	fmt.Printf("%v: event : %v\n", vio.Name, vio.stats[0])
	fmt.Printf("%v: status: %v\n", vio.Name, vio.stats[1])
}

// ExecuteDescriptorChain implements Handler.ExecuteDescriptorChain.
func (vio *Input) ExecuteDescriptorChain(vq *Queue, idx uint16) (
	uint32, error) {
	vio.Debugf("vq%v: chain: idx=%v", vq.Index, idx)

	vio.stats[vq.Index]++
	if vq.Index == 0 {
		vio.recvCh <- idx
		return 0, nil
	}

	desc, err := vq.loadDesc(idx)
	if err != nil {
		return 0, err
	}
	vio.Debugf("desc: %v", desc)

	// Status queue.
	var transmitted uint32
	for {
		buf, err := vio.guestData(desc.Addr, uint64(desc.Len))
		if err != nil {
			return 0, err
		}
		vio.Infof("statusq:\n%s", hex.Dump(buf))
		transmitted += desc.Len

		if desc.Flags&VIRTQ_DESC_F_NEXT == 0 {
			break
		}
		desc, err = vq.loadDesc(desc.Next)
		if err != nil {
			return 0, err
		}
	}
	return transmitted, nil
}

func (vio *Input) receiver(vq *Queue) {
	for desc := range vio.recvCh {

		ev := <-vio.eventCh

		tx, err := vio.storeEvent(vq, desc, ev)
		if err != nil {
			vio.Errorf("store event failed: %v", err)
		}
		vio.M.Lock()
		vio.CompleteDescriptor(vq, desc, tx)
		vio.M.Unlock()
	}
}

func (vio *Input) storeEvent(vq *Queue, idx uint16, ev *InputEvent) (
	uint32, error) {

	desc, err := vq.loadDesc(idx)
	if err != nil {
		return 0, err
	}
	vio.Debugf("desc: %v", desc)

	if desc.Len < 8 || desc.Flags&VIRTQ_DESC_F_WRITE == 0 {
		return 0, fmt.Errorf("invalid eventq desc: %v", desc)
	}
	buf, err := vio.guestData(desc.Addr, uint64(desc.Len))
	if err != nil {
		return 0, err
	}
	vioBO.PutUint16(buf[0:], ev.Type)
	vioBO.PutUint16(buf[2:], ev.Code)
	vioBO.PutUint32(buf[4:], ev.Value)

	vio.Debugf("vq%v: wrote 1 event [%vB]", vq.Index, desc.Len)

	return desc.Len, nil
}

var inputRegs = map[uint64]string{
	0x100: "select",
	0x101: "subsel",
	0x102: "size",
}

func (vio *Input) cfg() ([]byte, int) {
	var data []byte
	switch uint16(vio.sel)<<8 | uint16(vio.subsel) {
	case 0x0000: // VIRTIO_INPUT_CFG_UNSET
	case 0x0100: // VIRTIO_INPUT_CFG_ID_NAME
		data = []byte("GoEMU Input Device")
	case 0x0200: // VIRTIO_INPUT_CFG_ID_SERIAL
		data = []byte("424242")
	case 0x1100: // VIRTIO_INPUT_CFG_EV_BITS | 0 - supported categories
		data = inputCategoriesBitmap[:]
	case 0x1101: // VIRTIO_INPUT_CFG_EV_BITS | EV_KEY
		data = inputKeyBitmap[:]
	case 0x1103: // VIRTIO_INPUT_CFG_EV_BITS | EV_ABS
		data = inputAbsBitmap[:]
	case 0x1200: // VIRTIO_INPUT_CFG_ABS_INFO | ABS_X
		data = vio.absX
	case 0x1201: // VIRTIO_INPUT_CFG_ABS_INFO | ABS_Y
		data = vio.absY
	default:
		vio.Debugf("cfg: skipping: sel=%02x, subsel=%02x", vio.sel, vio.subsel)
	}

	return data, len(data)
}

func (vio *Input) Load8(paddr uint64) (uint8, error) {
	offset := paddr - vio.Start

	cfg, size := vio.cfg()

	switch offset {
	case 0x100:
		return vio.sel, nil
	case 0x101:
		return vio.subsel, nil
	case 0x102:
		return uint8(size), nil

	default:
		if 0x108 <= offset && offset+1 <= 0x108+uint64(size) {
			offset -= 0x108
			return cfg[offset], nil
		}

		return vio.MMIO.Load8(paddr)
	}
}

func (vio *Input) Load16(paddr uint64) (uint16, error) {
	offset := paddr - vio.Start

	cfg, size := vio.cfg()

	if 0x108 <= offset && offset+2 <= 0x108+uint64(size) {
		offset -= 0x108
		return vioBO.Uint16(cfg[offset:]), nil
	}
	return vio.MMIO.Load16(paddr)
}

func (vio *Input) Load32(paddr uint64) (uint32, error) {
	offset := paddr - vio.Start

	cfg, size := vio.cfg()

	if 0x108 <= offset && offset+4 <= 0x108+uint64(size) {
		offset -= 0x108
		return vioBO.Uint32(cfg[offset:]), nil
	}
	return vio.MMIO.Load32(paddr)
}

func (vio *Input) Load64(paddr uint64) (uint64, error) {
	offset := paddr - vio.Start

	cfg, size := vio.cfg()

	if 0x108 <= offset && offset+8 <= 0x108+uint64(size) {
		offset -= 0x108
		return vioBO.Uint64(cfg[offset:]), nil
	}
	return vio.MMIO.Load64(paddr)
}

func (vio *Input) Store8(paddr uint64, v uint8) error {
	offset := paddr - vio.Start

	reg, ok := inputRegs[offset]
	if ok {
		vio.Debugf("Store8(%v[0x%03x], %v)", reg, offset, v)
	}

	switch offset {
	case 0x100: // select
		vio.sel = v

	case 0x101: // subsel
		vio.subsel = v

	default:
		return vio.MMIO.Store8(paddr, v)
	}

	return nil
}

func (vio *Input) Store16(paddr uint64, v uint16) error {
	offset := paddr - vio.Start
	if offset >= 0x100 {
		vio.Errorf("Store16(%x,%v)", paddr, v)
	}
	return vio.MMIO.Store16(paddr, v)
}

func (vio *Input) Store32(paddr uint64, v uint32) error {
	offset := paddr - vio.Start
	if offset >= 0x100 {
		vio.Errorf("Store32(%x[0x%03x],%v)", paddr, offset, v)
	}
	return vio.MMIO.Store32(paddr, v)
}

func (vio *Input) Store64(paddr uint64, v uint64) error {
	offset := paddr - vio.Start
	if offset >= 0x100 {
		vio.Errorf("Store64(%x,%v)", paddr, v)
	}
	return vio.MMIO.Store64(paddr, v)
}

type InputListener interface {
	OnKeyRelease(key Key)
	OnKeyPress(key Key)
	OnKeyRepeat(key Key)
	OnButtonRelease(key MouseButton)
	OnButtonPress(key MouseButton)
	OnButtonRepeat(key MouseButton)
	OnMouseMove(x, y int32)
}

// OnKeyRelease implements InputListener.OnKeyRelease.
func (vio *Input) OnKeyRelease(key Key) {
	vio.keyEvent(uint16(key), 0)
}

// OnKeyPress implements InputListener.OnKeyPress.
func (vio *Input) OnKeyPress(key Key) {
	vio.keyEvent(uint16(key), 1)
}

// OnKeyRepeat implements InputListener.OnKeyRepeat.
func (vio *Input) OnKeyRepeat(key Key) {
	vio.keyEvent(uint16(key), 2)
}

// OnButtonRelease implements InputListener.OnButtonRelease.
func (vio *Input) OnButtonRelease(button MouseButton) {
	vio.keyEvent(uint16(button), 0)
}

// OnButtonPress implements InputListener.OnButtonPress.
func (vio *Input) OnButtonPress(button MouseButton) {
	vio.keyEvent(uint16(button), 1)
}

// OnButtonRepeat implements InputListener.OnButtonRepeat.
func (vio *Input) OnButtonRepeat(button MouseButton) {
	vio.keyEvent(uint16(button), 2)
}

// OnMouseMove implements InputListener.OnMouseMove
func (vio *Input) OnMouseMove(x, y int32) {
	vio.M.Lock()
	defer vio.M.Unlock()

	if !vio.DriverOK() {
		return
	}

	vio.addEvent(uint16(EV_ABS), ABS_X, uint32(x))
	vio.addEvent(uint16(EV_ABS), ABS_Y, uint32(y))
	vio.addEvent(uint16(EV_SYN), SYN_REPORT, 0)
	vio.ProcessQueue(0)
}

func (vio *Input) keyEvent(code uint16, value uint32) {
	vio.M.Lock()
	defer vio.M.Unlock()

	if !vio.DriverOK() {
		return
	}

	vio.addEvent(uint16(EV_KEY), code, value)
	vio.addEvent(uint16(EV_SYN), SYN_REPORT, 0)
	vio.ProcessQueue(0)
}

func (vio *Input) addEvent(typ, code uint16, value uint32) {
	vio.eventCh <- &InputEvent{
		Type:  typ,
		Code:  code,
		Value: value,
	}
}

// KeyEventValue represents the state action of a key event.
type KeyEventValue uint32

const (
	KeyRelease KeyEventValue = 0
	KeyPress   KeyEventValue = 1
	KeyRepeat  KeyEventValue = 2
)

const (
	SYN_REPORT    uint16 = 0
	SYN_CONFIG    uint16 = 1
	SYN_MT_REPORT uint16 = 2
	SYN_DROPPED   uint16 = 3
	SYN_MAX       uint16 = 0xf
)

type KeyEventType uint16

const (
	EV_SYN       KeyEventType = 0x00
	EV_KEY       KeyEventType = 0x01
	EV_REL       KeyEventType = 0x02
	EV_ABS       KeyEventType = 0x03
	EV_MSC       KeyEventType = 0x04
	EV_SW        KeyEventType = 0x05
	EV_LED       KeyEventType = 0x11
	EV_SND       KeyEventType = 0x12
	EV_REP       KeyEventType = 0x14
	EV_FF        KeyEventType = 0x15
	EV_PWR       KeyEventType = 0x16
	EV_FF_STATUS KeyEventType = 0x17
)

type Key uint16

const (
	KEY_RESERVED         Key = 0
	KEY_ESC              Key = 1
	KEY_1                Key = 2
	KEY_2                Key = 3
	KEY_3                Key = 4
	KEY_4                Key = 5
	KEY_5                Key = 6
	KEY_6                Key = 7
	KEY_7                Key = 8
	KEY_8                Key = 9
	KEY_9                Key = 10
	KEY_0                Key = 11
	KEY_MINUS            Key = 12
	KEY_EQUAL            Key = 13
	KEY_BACKSPACE        Key = 14
	KEY_TAB              Key = 15
	KEY_Q                Key = 16
	KEY_W                Key = 17
	KEY_E                Key = 18
	KEY_R                Key = 19
	KEY_T                Key = 20
	KEY_Y                Key = 21
	KEY_U                Key = 22
	KEY_I                Key = 23
	KEY_O                Key = 24
	KEY_P                Key = 25
	KEY_LEFTBRACE        Key = 26
	KEY_RIGHTBRACE       Key = 27
	KEY_ENTER            Key = 28
	KEY_LEFTCTRL         Key = 29
	KEY_A                Key = 30
	KEY_S                Key = 31
	KEY_D                Key = 32
	KEY_F                Key = 33
	KEY_G                Key = 34
	KEY_H                Key = 35
	KEY_J                Key = 36
	KEY_K                Key = 37
	KEY_L                Key = 38
	KEY_SEMICOLON        Key = 39
	KEY_APOSTROPHE       Key = 40
	KEY_GRAVE            Key = 41
	KEY_LEFTSHIFT        Key = 42
	KEY_BACKSLASH        Key = 43
	KEY_Z                Key = 44
	KEY_X                Key = 45
	KEY_C                Key = 46
	KEY_V                Key = 47
	KEY_B                Key = 48
	KEY_N                Key = 49
	KEY_M                Key = 50
	KEY_COMMA            Key = 51
	KEY_DOT              Key = 52
	KEY_SLASH            Key = 53
	KEY_RIGHTSHIFT       Key = 54
	KEY_KPASTERISK       Key = 55
	KEY_LEFTALT          Key = 56
	KEY_SPACE            Key = 57
	KEY_CAPSLOCK         Key = 58
	KEY_F1               Key = 59
	KEY_F2               Key = 60
	KEY_F3               Key = 61
	KEY_F4               Key = 62
	KEY_F5               Key = 63
	KEY_F6               Key = 64
	KEY_F7               Key = 65
	KEY_F8               Key = 66
	KEY_F9               Key = 67
	KEY_F10              Key = 68
	KEY_NUMLOCK          Key = 69
	KEY_SCROLLLOCK       Key = 70
	KEY_KP7              Key = 71
	KEY_KP8              Key = 72
	KEY_KP9              Key = 73
	KEY_KPMINUS          Key = 74
	KEY_KP4              Key = 75
	KEY_KP5              Key = 76
	KEY_KP6              Key = 77
	KEY_KPPLUS           Key = 78
	KEY_KP1              Key = 79
	KEY_KP2              Key = 80
	KEY_KP3              Key = 81
	KEY_KP0              Key = 82
	KEY_KPDOT            Key = 83
	KEY_ZENKAKUHANKAKU   Key = 85
	KEY_102ND            Key = 86
	KEY_F11              Key = 87
	KEY_F12              Key = 88
	KEY_RO               Key = 89
	KEY_KATAKANA         Key = 90
	KEY_HIRAGANA         Key = 91
	KEY_HENKAN           Key = 92
	KEY_KATAKANAHIRAGANA Key = 93
	KEY_MUHENKAN         Key = 94
	KEY_KPJPCOMMA        Key = 95
	KEY_KPENTER          Key = 96
	KEY_RIGHTCTRL        Key = 97
	KEY_KPSLASH          Key = 98
	KEY_SYSRQ            Key = 99
	KEY_RIGHTALT         Key = 100
	KEY_LINEFEED         Key = 101
	KEY_HOME             Key = 102
	KEY_UP               Key = 103
	KEY_PAGEUP           Key = 104
	KEY_LEFT             Key = 105
	KEY_RIGHT            Key = 106
	KEY_END              Key = 107
	KEY_DOWN             Key = 108
	KEY_PAGEDOWN         Key = 109
	KEY_INSERT           Key = 110
	KEY_DELETE           Key = 111
	KEY_MACRO            Key = 112
	KEY_MUTE             Key = 113
	KEY_VOLUMEDOWN       Key = 114
	KEY_VOLUMEUP         Key = 115
	KEY_POWER            Key = 116 /* SC System Power Down */
	KEY_KPEQUAL          Key = 117
	KEY_KPPLUSMINUS      Key = 118
	KEY_PAUSE            Key = 119
	KEY_SCALE            Key = 120 /* AL Compiz Scale (Expose) */
	KEY_KPCOMMA          Key = 121
	KEY_HANGEUL          Key = 122
	KEY_HANGUEL          Key = KEY_HANGEUL
	KEY_HANJA            Key = 123
	KEY_YEN              Key = 124
	KEY_LEFTMETA         Key = 125
	KEY_RIGHTMETA        Key = 126
	KEY_COMPOSE          Key = 127
	KEY_STOP             Key = 128 /* AC Stop */
	KEY_AGAIN            Key = 129
	KEY_PROPS            Key = 130 /* AC Properties */
	KEY_UNDO             Key = 131 /* AC Undo */
	KEY_FRONT            Key = 132
	KEY_COPY             Key = 133 /* AC Copy */
	KEY_OPEN             Key = 134 /* AC Open */
	KEY_PASTE            Key = 135 /* AC Paste */
	KEY_FIND             Key = 136 /* AC Search */
	KEY_CUT              Key = 137 /* AC Cut */
	KEY_HELP             Key = 138 /* AL Integrated Help Center */
	KEY_MENU             Key = 139 /* Menu (show menu) */
	KEY_CALC             Key = 140 /* AL Calculator */
	KEY_SETUP            Key = 141
	KEY_SLEEP            Key = 142 /* SC System Sleep */
	KEY_WAKEUP           Key = 143 /* System Wake Up */
	KEY_FILE             Key = 144 /* AL Local Machine Browser */
	KEY_SENDFILE         Key = 145
	KEY_DELETEFILE       Key = 146
	KEY_XFER             Key = 147
	KEY_PROG1            Key = 148
	KEY_PROG2            Key = 149
	KEY_WWW              Key = 150 /* AL Internet Browser */
	KEY_MSDOS            Key = 151
	KEY_COFFEE           Key = 152 /* AL Terminal Lock/Screensaver */
	KEY_SCREENLOCK       Key = KEY_COFFEE
	KEY_ROTATE_DISPLAY   Key = 153 /* Display orientation for e.g. tablets */
	KEY_DIRECTION        Key = KEY_ROTATE_DISPLAY
	KEY_CYCLEWINDOWS     Key = 154
	KEY_MAIL             Key = 155
	KEY_BOOKMARKS        Key = 156 /* AC Bookmarks */
	KEY_COMPUTER         Key = 157
	KEY_BACK             Key = 158 /* AC Back */
	KEY_FORWARD          Key = 159 /* AC Forward */
	KEY_CLOSECD          Key = 160
	KEY_EJECTCD          Key = 161
	KEY_EJECTCLOSECD     Key = 162
	KEY_NEXTSONG         Key = 163
	KEY_PLAYPAUSE        Key = 164
	KEY_PREVIOUSSONG     Key = 165
	KEY_STOPCD           Key = 166
	KEY_RECORD           Key = 167
	KEY_REWIND           Key = 168
	KEY_PHONE            Key = 169 /* Media Select Telephone */
	KEY_ISO              Key = 170
	KEY_CONFIG           Key = 171 /* AL Consumer Control Configuration */
	KEY_HOMEPAGE         Key = 172 /* AC Home */
	KEY_REFRESH          Key = 173 /* AC Refresh */
	KEY_EXIT             Key = 174 /* AC Exit */
	KEY_MOVE             Key = 175
	KEY_EDIT             Key = 176
	KEY_SCROLLUP         Key = 177
	KEY_SCROLLDOWN       Key = 178
	KEY_KPLEFTPAREN      Key = 179
	KEY_KPRIGHTPAREN     Key = 180
	KEY_NEW              Key = 181 /* AC New */
	KEY_REDO             Key = 182 /* AC Redo/Repeat */
	KEY_F13              Key = 183
	KEY_F14              Key = 184
	KEY_F15              Key = 185
	KEY_F16              Key = 186
	KEY_F17              Key = 187
	KEY_F18              Key = 188
	KEY_F19              Key = 189
	KEY_F20              Key = 190
	KEY_F21              Key = 191
	KEY_F22              Key = 192
	KEY_F23              Key = 193
	KEY_F24              Key = 194
	KEY_PLAYCD           Key = 200
	KEY_PAUSECD          Key = 201
	KEY_PROG3            Key = 202
	KEY_PROG4            Key = 203
	KEY_ALL_APPLICATIONS Key = 204 /* AC Desktop Show All Applications */
	KEY_DASHBOARD        Key = KEY_ALL_APPLICATIONS
	KEY_SUSPEND          Key = 205
	KEY_CLOSE            Key = 206 /* AC Close */
	KEY_PLAY             Key = 207
	KEY_FASTFORWARD      Key = 208
	KEY_BASSBOOST        Key = 209
	KEY_PRINT            Key = 210 /* AC Print */
	KEY_HP               Key = 211
	KEY_CAMERA           Key = 212
	KEY_SOUND            Key = 213
	KEY_QUESTION         Key = 214
	KEY_EMAIL            Key = 215
	KEY_CHAT             Key = 216
	KEY_SEARCH           Key = 217
	KEY_CONNECT          Key = 218
	KEY_FINANCE          Key = 219 /* AL Checkbook/Finance */
	KEY_SPORT            Key = 220
	KEY_SHOP             Key = 221
	KEY_ALTERASE         Key = 222
	KEY_CANCEL           Key = 223 /* AC Cancel */
	KEY_BRIGHTNESSDOWN   Key = 224
	KEY_BRIGHTNESSUP     Key = 225
	KEY_MEDIA            Key = 226
	KEY_SWITCHVIDEOMODE  Key = 227
	KEY_KBDILLUMTOGGLE   Key = 228
	KEY_KBDILLUMDOWN     Key = 229
	KEY_KBDILLUMUP       Key = 230
	KEY_SEND             Key = 231 /* AC Send */
	KEY_REPLY            Key = 232 /* AC Reply */
	KEY_FORWARDMAIL      Key = 233 /* AC Forward Msg */
	KEY_SAVE             Key = 234 /* AC Save */
	KEY_DOCUMENTS        Key = 235
	KEY_BATTERY          Key = 236
	KEY_BLUETOOTH        Key = 237
	KEY_WLAN             Key = 238
	KEY_UWB              Key = 239
	KEY_UNKNOWN          Key = 240
	KEY_VIDEO_NEXT       Key = 241 /* drive next video source */
	KEY_VIDEO_PREV       Key = 242 /* drive previous video source */
	KEY_BRIGHTNESS_CYCLE Key = 243 /* brightness up, after max is min */
	KEY_BRIGHTNESS_AUTO  Key = 244
	KEY_BRIGHTNESS_ZERO  Key = KEY_BRIGHTNESS_AUTO
	KEY_DISPLAY_OFF      Key = 245 /* display device to off state */
	KEY_WWAN             Key = 246 /* Wireless WAN (LTE, UMTS, GSM, etc.) */
	KEY_WIMAX            Key = KEY_WWAN
	KEY_RFKILL           Key = 247 /* Key that controls all radios */
	KEY_MICMUTE          Key = 248 /* Mute / unmute the microphone */
)

type MouseButton uint16

// Mouse buttons.
const (
	BTN_MOUSE   MouseButton = 0x110
	BTN_LEFT    MouseButton = 0x110
	BTN_RIGHT   MouseButton = 0x111
	BTN_MIDDLE  MouseButton = 0x112
	BTN_SIDE    MouseButton = 0x113
	BTN_EXTRA   MouseButton = 0x114
	BTN_FORWARD MouseButton = 0x115
	BTN_BACK    MouseButton = 0x116
	BTN_TASK    MouseButton = 0x117
)

const (
	REL_X      uint16 = 0x00
	REL_Y      uint16 = 0x01
	REL_Z      uint16 = 0x02
	REL_RX     uint16 = 0x03
	REL_RY     uint16 = 0x04
	REL_RZ     uint16 = 0x05
	REL_HWHEEL uint16 = 0x06
	REL_DIAL   uint16 = 0x07
	REL_WHEEL  uint16 = 0x08
	REL_MISC   uint16 = 0x09
)

const (
	ABS_X           uint16 = 0x00
	ABS_Y           uint16 = 0x01
	ABS_Z           uint16 = 0x02
	ABS_RX          uint16 = 0x03
	ABS_RY          uint16 = 0x04
	ABS_RZ          uint16 = 0x05
	ABS_THROTTLE    uint16 = 0x06
	ABS_RUDDER      uint16 = 0x07
	ABS_WHEEL       uint16 = 0x08
	ABS_GAS         uint16 = 0x09
	ABS_BRAKE       uint16 = 0x0a
	ABS_HAT0X       uint16 = 0x10
	ABS_HAT0Y       uint16 = 0x11
	ABS_HAT1X       uint16 = 0x12
	ABS_HAT1Y       uint16 = 0x13
	ABS_HAT2X       uint16 = 0x14
	ABS_HAT2Y       uint16 = 0x15
	ABS_HAT3X       uint16 = 0x16
	ABS_HAT3Y       uint16 = 0x17
	ABS_PRESSURE    uint16 = 0x18
	ABS_DISTANCE    uint16 = 0x19
	ABS_TILT_X      uint16 = 0x1a
	ABS_TILT_Y      uint16 = 0x1b
	ABS_TOOL_WIDTH  uint16 = 0x1c
	ABS_VOLUME      uint16 = 0x20
	ABS_PROFILE     uint16 = 0x21
	ABS_SND_PROFILE uint16 = 0x22
	ABS_MISC        uint16 = 0x28
)

var inputCategoriesBitmap = [128]byte{
	0x0a, // EV_KEY | EV_ABS
}

var inputKeyBitmap [128]byte

var inputRelBitmap = [128]byte{
	0x03, 0x01,
}

var inputAbsBitmap = [128]byte{
	0x03,
}

func setBit(v uint16) {
	inputKeyBitmap[v/8] |= 1 << (v % 8)
}

func init() {
	for _, v := range inputKeyMap {
		setBit(uint16(v))
	}
	for _, v := range mouseButtonMap {
		setBit(uint16(v))
	}
}

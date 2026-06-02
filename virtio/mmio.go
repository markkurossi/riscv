//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package virtio

import (
	"fmt"
)

const (
	Magic    = 0x74726976 // "virt"
	Version  = 0x2
	DeviceID = 0x2
	VendorID = 0x476f4d55 // "GoMU"

	FeatureAnyLayout = 1 << 28
	FeatureVersion1  = 1 << 31
)

var mmioRegs = map[uint64]string{
	0x000: "Magic",
	0x004: "Version",
	0x008: "DeviceID",
	0x00c: "VendorID",
	0x010: "DeviceFeatures",
	0x014: "DeviceFeaturesSel",
	0x020: "DriverFeatures",
	0x024: "DriverFeaturesSel",
	0x030: "QueueSel",
	0x034: "QueueNumMax",
	0x038: "QueueNum",
	0x044: "QueueReady",
	0x050: "QueueNotify",
	0x060: "InterruptStatus",
	0x064: "InterruptACK",
	0x070: "Status",
	0x080: "QueueDescLow",
	0x084: "QueueDescHigh",
	0x090: "QueueDriverLow",
	0x094: "QueueDriverHigh",
	0x0a0: "QueueDeviceLow",
	0x0a4: "QueueDeviceHigh",
	0x0fc: "ConfigGeneration",
	0x100: "CapacityLow",
	0x104: "CapacityHigh",
}

func mmioReg(ofs uint64) string {
	name, ok := mmioRegs[ofs]
	if ok {
		return fmt.Sprintf("%v[0x%03x]", name, ofs)
	}
	return fmt.Sprintf("0x%03x", ofs)
}

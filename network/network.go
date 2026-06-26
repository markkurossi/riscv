//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package network implements network protocol stack and interfaces.
package network

import (
	"encoding/binary"
	"fmt"
)

var (
	// BO defines the network byte-order.
	BO = binary.BigEndian
)

// MAC defines Ethernet address.
type MAC [6]byte

func (mac MAC) String() string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

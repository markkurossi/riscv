
virtio-net: DEBUG: UDP 0.0.0.0:68 -> 255.255.255.255:67
virtio-net: TRACE: IPv4:
00000000  45 10 01 48 00 00 00 00  80 11 39 96 00 00 00 00  |E..H......9.....|
00000010  ff ff ff ff 00 44 00 43  01 34 7c da 01 01 06 00  |.....D.C.4|.....|
00000020  29 a5 fc 68 00 07 00 00  00 00 00 00 00 00 00 00  |)..h............|
00000030  00 00 00 00 00 00 00 00  fe 53 c5 87 31 93 00 00  |.........S..1...|
00000040  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000050  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000060  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000070  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000080  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000090  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
000000a0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
000000b0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
000000c0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
000000d0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
000000e0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
000000f0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000100  00 00 00 00 00 00 00 00  63 82 53 63 35 01 01 3d  |........c.Sc5..=|
00000110  07 01 fe 53 c5 87 31 93  0c 07 66 72 65 65 62 73  |...S..1...freebs|
00000120  64 37 0a 01 1c 02 79 03  0f 06 0c 77 1a ff 00 00  |d7....y....w....|
00000130  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000140  00 00 00 00 00 00 00 00                           |........|


This hex dump contains a complete Layer 3 (IPv4) packet, inside of
which sits a Layer 4 (UDP) datagram, which carrying a Application
Layer (DHCP/BOOTP) payload. Specifically, this is a **DHCP Discover**
or **DHCP Request** broadcast message from a client machine.

Here is the step-by-step breakdown of the hex dump.

---

## 1. IPv4 Header (First 20 Bytes)

The first 20 bytes represent the standard IPv4 header:
`45 10 01 48 00 00 00 00 80 11 39 96 00 00 00 00 ff ff ff ff`

| Hex Bytes     | Field Name         | Decoded Value / Meaning         |
| ---           | ---                | ---                             |
| `45`          | Version & IHL      | **Version**: 4 (IPv4) <br>      |
|               |                    | **Header Length**: 5 words      |
| `10`          | Type of Service    | `0x10` (Minimize delay)         |
| `01 48`       | Total Length       | `0x0148` = **328 bytes**        |
| `00 00`       | Identification     | `0x0000`                        |
| `00 00`       | Flags & Frag Offs  | Don't Fragment / No offset      |
| `80`          | Time to Live (TTL) | 128                             |
| `11`          | Protocol           | `0x11` = **17 (UDP)**           |
| `39 96`       | Header Checksum    | `0x3996` (header integrity)     |
| `00 00 00 00` | Source IP Address  | **0.0.0.0** (no IP yet)         |
| `ff ff ff ff` | DestIP Address     | **255.255.255.255** (Broadcast) |
|               |                    |                                 |

---

## 2. UDP Header (8 Bytes)

Starting at offset `0x0014`, the UDP header handles the port multiplexing:
`00 44 00 43 01 34 7c da`

* **Source Port:** `00 44` $\rightarrow$ `68` (DHCP Client)
* **Destination Port:** `00 43` $\rightarrow$ `67` (DHCP Server)
* **Length:** `01 34` $\rightarrow$ 308 bytes (UDP Header + Payload)
* **Checksum:** `7c da` $\rightarrow$ `0x7CDA`

---

## 3. DHCP / BOOTP Payload (300 Bytes)

The remaining data is the DHCP application layer. Let's look at the
essential parameters extracted from the block starting at `01 01 06 00
...`:

### Core Fields

* **Message Type (Opcode):** `01` $\rightarrow$ Boot Request (Client to Server)
* **Hardware Type:** `01` $\rightarrow$ Ethernet (10Mb)
* **Hardware Address Length:** `06` $\rightarrow$ 6 bytes (Standard MAC address length)
* **Hops:** `00`
* **Transaction ID (XID):** `29 a5 fc 68` $\rightarrow$ `0x29A5FC68` (Used to match requests with responses)
* **Seconds Elapsed:** `00 07` $\rightarrow$ 7 seconds since the request process started
* **Client MAC Address (chaddr):** Located at offset `0x0038`: `fe 53 c5 87 31 93` $\rightarrow$ **`fe:53:c5:87:31:93`**

### DHCP Options

The options section always starts with the "Magic Cookie" `63 82 53
63` (at offset `0x0108`), which signals the start of DHCP options.

* **Option 53 (0x35): DHCP Message Type**
* `35 01 01` $\rightarrow$ Length: 1, Value: `01` (**DHCP Discover**)

* **Option 61 (0x3d): Client Identifier**
* `3d 07 01 fe 53 c5 87 31 93` $\rightarrow$ Length: 7, Value:
  Hardware type Ethernet (`01`) followed by the client MAC address.

* **Option 12 (0x0c): Host Name**
* `0c 07 66 72 65 65 62 73 64` $\rightarrow$ Length: 7, Value:
  **`freebsd`** (The client machine's hostname)

* **Option 55 (0x37): Parameter Request List**
* `37 0a ...` $\rightarrow$ The client is asking the server to provide
  specific network configuration configurations (like Subnet Mask
  `01`, Router `03`, Domain Name Server `06`, etc.).


* **End Mark:** `ff` signals the formal end of the options string.

---

| Message Type | L2 Src MAC | L2 Dst MAC        | L3 Src IP | L3 Dst IP       |
|--------------|------------|-------------------|-----------|-----------------|
| DHCPDISCOVER | Client MAC | ff:ff:ff:ff:ff:ff | 0.0.0.0   | 255.255.255.255 |
| DHCPOFFER    | Server MAC | Client MAC        | Server IP | 255.255.255.255 |
| DHCPREQUEST  | Client MAC | ff:ff:ff:ff:ff:ff | 0.0.0.0   | 255.255.255.255 |
| DHCPACK      | Server MAC | Client MAC        | Server IP | 255.255.255.255 |

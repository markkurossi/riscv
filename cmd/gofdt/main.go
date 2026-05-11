package main

import (
	"fmt"
	"log"
	"os"

	"github.com/markkurossi/gofdt"
)

func main() {
	// 1. Initialize the FDT builder with a buffer
	buf := make([]byte, 4096)
	fdt := gofdt.NewFDT(buf)

	// 2. Start Root Node
	fdt.BeginNode("")
	// Basic system properties
	fdt.PropStr("model", "goemu,riscv-emulator")
	fdt.PropStr("compatible", "riscv-virtio")
	fdt.PropU32("#address-cells", 2) // 64-bit addresses (2x32-bit)
	fdt.PropU32("#size-cells", 2)    // 64-bit sizes

	// 3. CPU Topology
	fdt.BeginNode("cpus")
	fdt.PropU32("#address-cells", 1)
	fdt.PropU32("#size-cells", 0)
	fdt.PropU32("timebase-frequency", 10000000) // 10MHz standard

	fdt.BeginNode("cpu@0")
	fdt.PropStr("device_type", "cpu")
	fdt.PropU32("reg", 0)
	fdt.PropStr("status", "okay")
	fdt.PropStr("compatible", "riscv")
	fdt.PropStr("riscv,isa", "rv64imafdc")
	fdt.PropStr("mmu-type", "riscv,sv39")

	// CPU Interrupt Controller (required for S-Mode)
	fdt.BeginNode("interrupt-controller")
	fdt.PropU32("#interrupt-cells", 1)
	fdt.Prop("interrupt-controller", nil, 0)
	fdt.PropStr("compatible", "riscv,cpu-intc")
	fdt.PropU32("phandle", 1) // Reference handle for PLIC
	fdt.EndNode()
	fdt.EndNode()
	fdt.EndNode()

	// 4. System Memory (RAM)
	// Assuming RAM starts at 0x80000000 and is 512MB
	fdt.BeginNodeNum("memory", 0x80000000)
	fdt.PropStr("device_type", "memory")
	// reg is [address_high, address_low, size_high, size_low]
	tab := [4]uint32{
		0x0,
		0x80000000,
		0x0,
		0x20000000,
	}
	fdt.PropTabU32("reg", &tab[0], 4)
	fdt.EndNode()

	// 5. Peripherals (UART 16550A)
	fdt.BeginNode("uart@10000000")
	fdt.PropStr("compatible", "ns16550a")
	tab = [4]uint32{
		0x0, 0x10000000, 0x0, 0x100,
	}
	fdt.PropTabU32("reg", &tab[0], 4)
	fdt.PropU32("clock-frequency", 3686400)
	fdt.PropU32("interrupt-parent", 1) // Link to CPU INTC phandle
	fdt.PropU32("interrupts", 10)
	fdt.EndNode()

	// 6. Chosen node (Boot arguments)
	fdt.BeginNode("chosen")
	// Tells Linux to use the UART for its console
	fdt.PropStr("bootargs", "console=ttyS0 earlycon=uart8250,mmio,0x10000000")
	fdt.PropStr("stdout-path", "/uart@10000000")
	fdt.EndNode()

	fdt.EndNode() // End Root

	// 7. Finish and get binary blob
	size := fdt.Output()
	dtb := buf[:size]

	fmt.Printf("Generated DTB: %d bytes\n", len(dtb))

	// Write to file for verification (optional)
	err := os.WriteFile("test.dtb", dtb, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

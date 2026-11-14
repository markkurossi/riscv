// Package riscv provides common utilities and resources for the
// RISC-V emulator implemented in Go.
//
// The project implements an RV64GC RISC-V emulator capable of booting
// Linux, NetBSD, FreeBSD, and OpenBSD. It provides CPU emulation,
// virtual memory, interrupt handling, device emulation, and VirtIO
// support.
//
// The top-level package contains shared utilities used by the
// emulator and its supporting tools. The implementation is organized
// into dedicated packages:
//
//   - cpu implements the RISC-V processor.
//   - isa defines the RISC-V instruction set architecture.
//   - emulator provides Linux userspace emulation.
//   - kernel implements the emulator kernel and execution environment.
//   - mmu implements virtual memory and address translation.
//   - memory defines physical memory interfaces.
//   - dev implements memory-mapped devices.
//   - virtio implements VirtIO devices.
//   - network provides networking protocols and interfaces.
//   - linux provides Linux syscall interfaces and types.
//   - logger provides debugging and tracing facilities.
//
// The emulator is designed to be readable and hackable, with an
// emphasis on correctness, maintainability, and practical
// experimentation with RISC-V architecture and operating system
// internals.
//
// See the project README for build instructions, supported operating
// systems, examples, and additional documentation.
package riscv

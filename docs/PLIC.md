#  RISC-V Platform-Level Interrupt Controller Specification (PLIC)

The extracts below are from the original
[PLIC](https://github.com/riscv/riscv-plic-spec) specification.

The original copyright is as follows:

```
This RISC-V PLIC specification is © 2017-2023 RISC-V international

This document is released under a Creative Commons Attribution 4.0 International License.
https://creativecommons.org/licenses/by/4.0/.

Please cite as: “RISC-V Platform-Level Interrupt Controller Specification", RISC-V International

This document is a derivative of the "The RISC-V Instruction Set
Manual, Volume II: Privileged Architecture, Document version 1.9.1"
released under following license: © 2010–2017 Andrew Waterman, Yunsup
Lee, Rimas Aviˇzienis, David Patterson, Krste Asanovi ́c. Creative
Commons Attribution 4.0 International License.
```

## 1.2. Interrupt Gateway

The interrupt gateways are responsible for converting global interrupt
signals into a common interrupt request format, and for controlling
the flow of interrupt requests to the PLIC core. At most one interrupt
request per interrupt source can be pending in the PLIC core at any
time, indicated by setting the source’s IP bit. The gateway only
forwards a new interrupt request to the PLIC core after receiving
notification that the interrupt handler servicing the previous
interrupt request from the same source has completed.

If the global interrupt source uses level-sensitive interrupts, the
gateway will convert the first assertion of the interrupt level into
an interrupt request, but thereafter the gateway will not forward an
additional interrupt request until it receives an interrupt completion
message. On receiving an interrupt completion message, if the
interrupt is level-triggered and the interrupt is still asserted, a
new interrupt request will be forwarded to the PLIC core. The gateway
does not have the facility to retract an interrupt request once
forwarded to the PLIC core. If a level-sensitive interrupt source
deasserts the interrupt after the PLIC core accepts the request and
before the interrupt is serviced, the interrupt request remains
present in the IP bit of the PLIC core and will be serviced by a
handler, which will then have to determine that the interrupt device
no longer requires service.

If the global interrupt source was edge-triggered, the gateway will
convert the first matching signal edge into an interrupt
request. Depending on the design of the device and the interrupt
handler, in between sending an interrupt request and receiving notice
of its handler’s completion, the gateway might either ignore
additional matching edges or increment a counter of pending
interrupts. In either case, the next interrupt request will not be
forwarded to the PLIC core until the previous completion message has
been received. If the gateway has a pending interrupt counter, the
counter will be decremented when the interrupt request is accepted by
the PLIC core.

## 1.3. Interrupt Notifications

Each interrupt target has an external interrupt pending (EIP) bit in
the PLIC core that indicates that the corresponding target has a
pending interrupt waiting for service. The value in EIP can change as
a result of changes to state in the PLIC core, brought on by interrupt
sources, interrupt targets, or other agents manipulating register
values in the PLIC. The value in EIP is communicated to the
destination target as an interrupt notification. If the target is a
RISC-V hart context, the interrupt notifications arrive on the
meip/seip bits depending on the privilege level of the hart context.

## Flowchart

| Source      | Gateway   | PLIC Core        | Target       | Handler |
|-------------|-----------|------------------|--------------|---------|
| signalled-> |           |                  |              |         |
|             | Request-> |                  |              |         |
|             |           | Notification->   |              |         |
|             |           |                  | <-Claim      |         |
|             |           | Claim Response-> |              | Running |
|             |           |                  |              | ...     |
|             |           |                  |              | Done    |
|             |           |                  | <-Completion |         |
|             |           | <-Completion     |              |         |
|             | Request-> |                  |              |         |

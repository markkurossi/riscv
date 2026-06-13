# PE/COFF Header

| Offset | Size | Field Name  | Expected/Typical Value                    |
| ------ | ---- | ----------  | ----------------------                    |
| 0x00   | 4    | code0       | 0x006F0041 (Compr)or 0xNNNNNN6F (Uncompr) |
| 0x04   | 4    | code1       | Instruction bytes                         |
| 0x08   | 8    | text_offset | 0x00200000 (Typical 2MB)                  |
| 0x10   | 8    | image_size  | Varies (e.g., 0x00357750)                 |
| 0x18   | 8    | flags       | 0x0000000000000000                        |
| 0x20   | 4    | version     | 0x00000002 (v0.2)                         |
| 0x24   | 4    | res1        | 0x00000000                                |
| 0x28   | 8    | res2        | 0x0000000000000000                        |
| 0x30   | 8    | magic       | 0x205643534952 (ASCII: "RISCV   ")        |
| 0x38   | 4    | magic2      | 0x05435352 (ASCII: "RSC\x05)              |
| 0x3C   | 4    | res3        | Pointer Offset (e.g., 0x00000040)"        |

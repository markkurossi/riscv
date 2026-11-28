//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math/bits"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/tabulate"
)

type InstrEntry struct {
	Encoding       string   `json:"encoding"`
	Extension      []string `json:"extension"`
	Mask           string   `json:"mask"`
	Match          string   `json:"match"`
	VariableFields []string `json:"variable_fields"`
}

type BitRange struct {
	MSB int
	LSB int
}

type NamedRange struct {
	Name  string
	Fixed string
	BitRange
}

func (br BitRange) String() string {
	if br.MSB == br.LSB {
		return fmt.Sprintf("%v", br.MSB)
	}
	return fmt.Sprintf("%v:%v", br.MSB, br.LSB)
}

type SortedInstr struct {
	Name     string
	Enum     string
	Encoding string
	Is16     bool
	IsVPU    bool
	IsFPU    bool
	Mask     uint32
	Match    uint32
	PopCnt   int
	Fields   []string
}

func (si SortedInstr) FixedRange(msb, lsb int) NamedRange {
	from := 31 - msb
	to := 31 - lsb

	fixed := si.Encoding[from : to+1]

	val, err := strconv.ParseUint(fixed, 2, 32)
	if err != nil {
		fmt.Printf("%v: invalid fixed value %v:%v=%v", si.Name, msb, lsb, fixed)
	}

	var name string

	if msb == 31 && lsb == 26 {
		name = "func6"
	} else if msb == 31 && lsb == 25 {
		name = "func7"
	} else if msb == 14 && lsb == 12 {
		if si.IsVPU {
			name = "width"
		} else {
			name = "func3"
		}
	} else if msb == 6 && lsb == 0 {
		name = isa.Group(val).String()
	} else {
		name = "???"
	}

	return NamedRange{
		Name:  name,
		Fixed: fixed,
		BitRange: BitRange{
			MSB: msb,
			LSB: lsb,
		},
	}
}

func toMnemonic(name string) string {
	return strings.ReplaceAll(name, "_", ".")
}

func toEnumName(name string) string {
	parts := strings.Split(name, "_")
	for idx, part := range parts {
		parts[idx] = strings.Title(part)
	}
	return strings.Join(parts, "")
}

func parseHex32(s string) uint32 {
	v, err := strconv.ParseUint(strings.TrimPrefix(s, "0x"), 16, 32)
	if err != nil {
		panic(err)
	}
	return uint32(v)
}

func loadArgLUT(path string) map[string]BitRange {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		panic(err)
	}
	lut := make(map[string]BitRange, len(rows))
	for _, r := range rows {
		if len(r) < 3 {
			continue
		}
		name := strings.TrimSpace(r[0])
		msb, err1 := strconv.Atoi(strings.TrimSpace(r[1]))
		lsb, err2 := strconv.Atoi(strings.TrimSpace(r[2]))
		if err1 == nil && err2 == nil {
			lut[name] = BitRange{MSB: msb, LSB: lsb}
		}
	}
	return lut
}

var RVA23S64 = map[string]bool{
	"rv64_a":         true,
	"rv64_c":         true,
	"rv64_d":         true,
	"rv64_f":         true,
	"rv64_h":         true,
	"rv64_i":         true,
	"rv64_m":         true,
	"rv64_zbb":       true,
	"rv_a":           true,
	"rv_c":           true,
	"rv_d":           true,
	"rv_f":           true,
	"rv_h":           true,
	"rv_i":           true,
	"rv_m":           true,
	"rv_s":           true,
	"rv_svinval":     true,
	"rv_v":           true,
	"rv_zawrs":       true,
	"rv_zba":         true,
	"rv_zbb":         true,
	"rv_zbs":         true,
	"rv_zcb":         true,
	"rv_zcmop":       false,
	"rv_zfa":         true,
	"rv_zfhmin":      true,
	"rv_zicbom":      true,
	"rv_zicbop":      true,
	"rv_zicboz":      true,
	"rv_zicond":      true,
	"rv_zicsr":       true,
	"rv_zifencei":    true,
	"rv_zihintntl":   false,
	"rv_zihintpause": false,
	"rv_zimop":       false,
	"rv_zvbb":        true,
	"rv_zvfhmin":     true,
}

var FPU = map[string]bool{
	"rv_f":       true,
	"rv64_f":     true,
	"rv_d":       true,
	"rv64_d":     true,
	"rv_zfa":     true,
	"rv_zfhmin":  true,
	"rv_zvfhmin": true,
}

var extensions = make(map[string]int)
var list []SortedInstr
var lut map[string]BitRange

func main() {
	flag.Parse()

	rawJSON, err := os.ReadFile("instr_dict.json")
	if err != nil {
		panic(err)
	}
	var dict map[string]InstrEntry
	if err := json.Unmarshal(rawJSON, &dict); err != nil {
		panic(err)
	}
	lut = loadArgLUT("arg_lut.csv")

	fieldSet := map[string]bool{}

	var total int

	for name, entry := range dict {
		// Skip pseudo-ops and RV32 instructions.
		var pseudo, rv32, vpu, fpu, rva23s64 bool
		for _, ext := range entry.Extension {
			if RVA23S64[ext] {
				rva23s64 = true
			}
			if FPU[ext] {
				fpu = true
			}
			if strings.Contains(ext, "pseudo") {
				pseudo = true
			}
			if strings.HasPrefix(ext, "rv32_") {
				rv32 = true
			}
			if strings.HasPrefix(ext, "rv_zv") ||
				strings.HasPrefix(ext, "rv_v") {
				vpu = true
			}
		}
		if pseudo || rv32 || !rva23s64 {
			continue
		}
		total++
		if vpu {
			count := extensions["vpu"]
			extensions["vpu"] = count + 1
		} else if fpu {
			count := extensions["fpu"]
			extensions["fpu"] = count + 1
		} else {
			for _, ext := range entry.Extension {
				count := extensions[ext]
				extensions[ext] = count + 1
			}
		}

		is16 := len(entry.Encoding) == 16
		mask := parseHex32(entry.Mask)
		match := parseHex32(entry.Match)
		for _, vf := range entry.VariableFields {
			fieldSet[vf] = true
		}
		list = append(list, SortedInstr{
			Name:     toMnemonic(name),
			Enum:     toEnumName(name),
			Encoding: entry.Encoding,
			Is16:     is16,
			IsVPU:    vpu,
			IsFPU:    fpu,
			Mask:     mask,
			Match:    match,
			PopCnt:   bits.OnesCount32(mask),
			Fields:   entry.VariableFields,
		})
	}

	// Sort most-specific mask first, then deterministic by name
	sort.Slice(list, func(i, j int) bool {
		if list[i].PopCnt != list[j].PopCnt {
			return list[i].PopCnt > list[j].PopCnt
		}
		return list[i].Name < list[j].Name
	})

	if len(flag.Args()) > 0 {
		decodeInstructions(flag.Args())
		return
	}

	for ext, count := range extensions {
		fmt.Printf("%v:\t%v\n", ext, count)
	}
	numFPU := extensions["fpu"]
	numRVV := extensions["vpu"]

	fmt.Printf("FPU  : %v\n", numFPU)
	fmt.Printf("VPU  : %v\n", numRVV)
	fmt.Printf("rest : %v\n", total-numFPU-numRVV)
	fmt.Printf("total: %v\n", total)

	fmt.Println("// Code generated by gen_decoder.go; DO NOT EDIT.")
	fmt.Println("package riscvdec")
	fmt.Println()
	fmt.Println("type Opcode uint8")
	fmt.Println()
	fmt.Println("// RISC-V base instruction opcodes.")
	fmt.Println("const (")
	fmt.Println("\tInvalid Op = iota")
	for _, ins := range list {
		if ins.Is16 || ins.IsVPU || ins.IsFPU {
			continue
		}
		fmt.Printf("\t%s\n", ins.Enum)
	}
	fmt.Println(")")
	fmt.Println()

	fmt.Println("// Floating point instruction opcodes.")
	fmt.Println("const (")
	for idx, ins := range list {
		if ins.Is16 || !ins.IsFPU {
			continue
		}
		if idx == 0 {
			fmt.Printf("\t%s FpuOp = iota\n", ins.Enum)
		} else {
			fmt.Printf("\t%s\n", ins.Enum)
		}
	}
	fmt.Println(")")
	fmt.Println()

	fmt.Println("// Vector extension opcodes.")
	fmt.Println("const (")
	for idx, ins := range list {
		if ins.Is16 || !ins.IsVPU {
			continue
		}
		if idx == 0 {
			fmt.Printf("\t%s VpuOp = iota\n", ins.Enum)
		} else {
			fmt.Printf("\t%s\n", ins.Enum)
		}
	}
	fmt.Println(")")
	fmt.Println()

	// Emit raw field extractors from arg_lut.csv
	var allFields []string
	for f := range fieldSet {
		allFields = append(allFields, f)
	}
	sort.Strings(allFields)

	fmt.Println("type RawFields struct {")
	for _, f := range allFields {
		fmt.Printf("\t%s uint32\n", sanitizeIdent(f))
	}
	fmt.Println("}")
	fmt.Println()

	fmt.Println("type Decoded struct {")
	fmt.Println("\tOp     Opcode")
	fmt.Println("\tName   string")
	fmt.Println("\tLen    uint8 // 2 or 4 bytes")
	fmt.Println("\tRaw    uint32")
	fmt.Println("\tFields RawFields")
	fmt.Println("}")
	fmt.Println()

	// Emit 16-bit and 32-bit decode tables
	fmt.Println("type decodeRule struct {")
	fmt.Println("\tmask   uint32")
	fmt.Println("\tmatch  uint32")
	fmt.Println("\top     Opcode")
	fmt.Println("\tname   string")
	fmt.Println("\textract func(uint32, *RawFields)")
	fmt.Println("}")
	fmt.Println()

	// Emit per-instruction field extractors
	for _, ins := range list {
		if len(ins.Fields) == 0 {
			continue
		}
		fmt.Printf("func extract_%s(inst uint32, f *RawFields) {\n", ins.Enum)
		for _, vf := range ins.Fields {
			br, ok := lut[vf]
			if !ok {
				continue
			}
			width := br.MSB - br.LSB + 1
			mask := (uint32(1) << width) - 1
			fmt.Printf("\tf.%s = (inst >> %d) & 0x%x\n", sanitizeIdent(vf), br.LSB, mask)
		}
		fmt.Println("}")
	}
	fmt.Println()

	emitTable := func(varName string, want16 bool) {
		fmt.Printf("var %s = []decodeRule{\n", varName)
		for _, ins := range list {
			if ins.Is16 != want16 {
				continue
			}
			extFn := "nil"
			if len(ins.Fields) > 0 {
				extFn = "extract_" + ins.Enum
			}
			fmt.Printf("\t{0x%08x, 0x%08x, %s, %q, %s},\n", ins.Mask, ins.Match, ins.Enum, ins.Name, extFn)
		}
		fmt.Println("}")
		fmt.Println()
	}

	emitTable("rules16", true)
	emitTable("rules32", false)

	fmt.Println(`// Decode decodes a 16-bit (in lower halfword) or 32-bit RISC-V instruction.
func Decode(inst uint32) Decoded {
	if (inst & 0x3) != 0x3 {
		inst16 := inst & 0xffff
		for i := range rules16 {
			r := &rules16[i]
			if (inst16 & r.mask) == r.match {
				d := Decoded{Op: r.op, Name: r.name, Len: 2, Raw: inst16}
				if r.extract != nil {
					r.extract(inst16, &d.Fields)
				}
				return d
			}
		}
		return Decoded{Op: OP_INVALID, Name: "invalid", Len: 2, Raw: inst16}
	}

	for i := range rules32 {
		r := &rules32[i]
		if (inst & r.mask) == r.match {
			d := Decoded{Op: r.op, Name: r.name, Len: 4, Raw: inst}
			if r.extract != nil {
				r.extract(inst, &d.Fields)
			}
			return d
		}
	}
	return Decoded{Op: OP_INVALID, Name: "invalid", Len: 4, Raw: inst}
}`)
}

func sanitizeIdent(s string) string {
	s = strings.ReplaceAll(s, ".", "_")
	return "F_" + s
}

func decodeInstructions(args []string) error {
	for _, arg := range flag.Args() {
		raw, err := strconv.ParseUint(arg, 16, 32)
		if err != nil {
			return fmt.Errorf("invalid instruction '%v': %w", arg, err)
		}

		for _, instr := range list {
			if uint32(raw)&instr.Mask == instr.Match {
				var attrs []string
				var hex string
				if instr.Is16 {
					attrs = append(attrs, "16-bit")
					hex = fmt.Sprintf("%04x", raw)
				} else {
					hex = fmt.Sprintf("%08x", raw)
				}
				if instr.IsVPU {
					attrs = append(attrs, "VPU")
				}
				if instr.IsFPU {
					attrs = append(attrs, "FPU")
				}

				fmt.Printf("%v: %v %v %v\n",
					hex, instr.Name, instr.Enum, strings.Join(attrs, ", "))

				// Sort instr.Fields from MSB to LSB.
				var ifields []NamedRange
				for _, field := range instr.Fields {
					br, ok := lut[field]
					if !ok {
						return fmt.Errorf("%v: unknown field %v",
							instr.Name, field)
					}
					ifields = append(ifields, NamedRange{
						Name:     field,
						BitRange: br,
					})
				}
				sort.Slice(ifields, func(i, j int) bool {
					return ifields[i].MSB > ifields[j].MSB
				})

				var fields []NamedRange

				bit := 31
				if instr.Is16 {
					bit = 15
				}

				for _, ifield := range ifields {
					if ifield.MSB < bit {
						fields = append(fields,
							instr.FixedRange(bit, ifield.MSB+1))
					}
					fields = append(fields, ifield)
					bit = ifield.LSB - 1
				}
				if bit > 0 {
					fields = append(fields, instr.FixedRange(bit, 0))
				}

				tab := tabulate.New(tabulate.UnicodeLight)

				for _, f := range fields {
					tab.Header(f.BitRange.String()).SetAlign(tabulate.TC)
				}
				row := tab.Row()
				for _, f := range fields {
					if len(f.Fixed) > 0 {
						row.Column(f.Fixed)
					} else {
						row.Column(f.Name)
					}
				}
				row = tab.Row()
				for _, f := range fields {
					if len(f.Fixed) > 0 {
						row.Column(f.Name)
					} else {
						row.Column("")
					}
				}
				tab.Print(os.Stdout)
			}
		}
	}

	return nil
}

/*
 * These implementations are copied and modified from the Spike RISC-V
 * ISA Simulator
 *
 *   https://github.com/riscv-software-src/riscv-isa-sim
 *
 * The original copyright is as follows:
 *
 * Copyright (c) 2010-2017, The Regents of the University of California
 * (Regents).  All Rights Reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are met:
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 * 3. Neither the name of the Regents nor the
 *    names of its contributors may be used to endorse or promote products
 *    derived from this software without specific prior written permission.
 *
 * IN NO EVENT SHALL REGENTS BE LIABLE TO ANY PARTY FOR DIRECT,
 * INDIRECT, SPECIAL, INCIDENTAL, OR CONSEQUENTIAL DAMAGES, INCLUDING
 * LOST PROFITS, ARISING OUT OF THE USE OF THIS SOFTWARE AND ITS
 * DOCUMENTATION, EVEN IF REGENTS HAS BEEN ADVISED OF THE POSSIBILITY
 * OF SUCH DAMAGE.
 *
 * REGENTS SPECIFICALLY DISCLAIMS ANY WARRANTIES, INCLUDING, BUT NOT LIMITED TO,
 * THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR
 * PURPOSE. THE SOFTWARE AND ACCOMPANYING DOCUMENTATION, IF ANY, PROVIDED
 * HEREUNDER IS PROVIDED "AS IS". REGENTS HAS NO OBLIGATION TO PROVIDE
 * MAINTENANCE, SUPPORT, UPDATES, ENHANCEMENTS, OR MODIFICATIONS.
 */

int
vsetvli()
{
  set_vl(insn.rd(), insn.rs1(), RS1, insn.v_zimm11());
}

void
set_vl(int rd, int rs1, reg_t reqVL, reg_t newType)
{
  if (vtype->read() != newType) {
    int new_vlmul = int8_t(extract64(newType, 0, 3) << 5) >> 5;
    auto old_vlmax = vlmax;

    vsew = 1 << (extract64(newType, 3, 3) + 3);
    vflmul = new_vlmul >= 0 ? 1 << new_vlmul : 1.0 / (1 << -new_vlmul);
    vlmax = (VLEN/vsew) * vflmul;
    vta = extract64(newType, 6, 1);
    vma = extract64(newType, 7, 1);
    altfmt = extract64(newType, 8, 1);

    bool ill_altfmt = true;
    if (altfmt) {
      if (p->extension_enabled(EXT_ZVQWBDOTA8I) && vsew == 8)
        ill_altfmt = false;
      else if (p->extension_enabled(EXT_ZVQWBDOTA16I) && vsew == 16)
        ill_altfmt = false;
      else if (p->extension_enabled(EXT_ZVFQWBDOTA8F) && vsew == 8)
        ill_altfmt = false;
      else if (p->extension_enabled(EXT_ZVFWBDOTA16BF) && vsew == 16)
        ill_altfmt = false;
      else if (p->extension_enabled(EXT_ZVQWDOTA8I) && vsew == 8)
        ill_altfmt = false;
      else if (p->extension_enabled(EXT_ZVQWDOTA16I) && vsew == 16)
        ill_altfmt = false;
      else if (p->extension_enabled(EXT_ZVFQWDOTA8F) && vsew == 8)
        ill_altfmt = false;
      else if (p->extension_enabled(EXT_ZVFWDOTA16BF) && vsew == 16)
        ill_altfmt = false;
      else if (p->extension_enabled(EXT_ZVFBFA) && (vsew == 16 || vsew == 8))
        ill_altfmt = false;
      else if (p->extension_enabled(EXT_ZVFOFP8MIN) && vsew == 8)
        ill_altfmt = false;
      else if (p->extension_enabled(EXT_ZVTI8I32MM) && vsew == 8)
        ill_altfmt = false;
      else if (p->extension_enabled(EXT_ZVTOFP8FMM) && vsew == 8)
        ill_altfmt = false;
      else if (p->extension_enabled(EXT_ZVTBF16FMM) && vsew == 16)
        ill_altfmt = false;
    }

    vill = !(vflmul >= 0.125 && vflmul <= 8)
           || vsew > std::min(vflmul, 1.0f) * ELEN
           || (newType >> 9) != 0
           || (altfmt && ill_altfmt)
           || (rd == 0 && rs1 == 0 && old_vlmax != vlmax);

    if (vill) {
      vlmax = 0;
      vtype->write_raw(UINT64_MAX << (p->get_xlen() - 1));
    } else {
      vtype->write_raw(newType);
    }
  }

  // clear mtype
  widen = 0;
  tm = 0;
  tk = 0;
  mtype->write_raw(read_mtype());

  // set vl
  if (vlmax == 0) {
    vl->write_raw(0);
  } else if (rd == 0 && rs1 == 0) {
    ; // retain current VL
  } else if (rd != 0 && rs1 == 0) {
    vl->write_raw(vlmax);
  } else if (rs1 != 0) {
    vl->write_raw(std::min(reqVL, vlmax));
  }

  return vl->read();
}

int
vmv_v_i()
{
  require_vm;
  if (P.VU.vflmul > 1)
    {
      require_align(insn.rd(), P.VU.vflmul);
      require_align(insn.rs2(), P.VU.vflmul);
      if (false) {
        require_align(insn.rs1(), P.VU.vflmul);
      }
    }
  require(P.VU.vsew >= e8 && P.VU.vsew <= e64);
  require_vector(true);
  reg_t vl = P.VU.vl->read();
  reg_t sew = P.VU.vsew;
  reg_t rd_num = insn.rd();
  reg_t rs1_num = insn.rs1();
  reg_t rs2_num = insn.rs2();
  for (reg_t i = P.VU.vstart->read(); i < vl; ++i)
    {
      bool use_first = P.VU.mask_elt(0, i);

      if (sew == e8)
        {
          type_sew_t<e8>::type &vd = P.VU.elt<type_sew_t<e8>::type>(rd_num, i, true);
          type_sew_t<e8>::type simm5 = (type_sew_t<e8>::type)insn.v_simm5();
          type_sew_t<e8>::type vs2 = P.VU.elt<type_sew_t<e8>::type>(rs2_num, i);
          vd = simm5;
        }
      else if (sew == e16)
        {
          type_sew_t<e16>::type &vd = P.VU.elt<type_sew_t<e16>::type>(rd_num, i, true);
          type_sew_t<e16>::type simm5 = (type_sew_t<e16>::type)insn.v_simm5();
          type_sew_t<e16>::type vs2 = P.VU.elt<type_sew_t<e16>::type>(rs2_num, i);
          vd = simm5;
        }
      else if (sew == e32)
        {
          type_sew_t<e32>::type &vd = P.VU.elt<type_sew_t<e32>::type>(rd_num, i, true);
          type_sew_t<e32>::type simm5 = (type_sew_t<e32>::type)insn.v_simm5();
          type_sew_t<e32>::type vs2 = P.VU.elt<type_sew_t<e32>::type>(rs2_num, i);
          vd = simm5;
        }
      else if (sew == e64)
        {
          type_sew_t<e64>::type &vd = P.VU.elt<type_sew_t<e64>::type>(rd_num, i, true);
          type_sew_t<e64>::type simm5 = (type_sew_t<e64>::type)insn.v_simm5();
          type_sew_t<e64>::type vs2 = P.VU.elt<type_sew_t<e64>::type>(rs2_num, i);
          vd = simm5;
        }
    }

  P.VU.vstart->write(0)
}

// vector element for various SEW
template<typename T> T& elt(reg_t vReg, reg_t n, bool is_write = false) {
  assert(vsew != 0);
  assert((VLEN >> 3)/sizeof(T) > 0);
  reg_t elts_per_reg = (VLEN >> 3) / (sizeof(T));
  vReg += n / elts_per_reg;
  n = n % elts_per_reg;
#ifdef WORDS_BIGENDIAN
  // "V" spec 0.7.1 requires lower indices to map to lower significant
  // bits when changing SEW, thus we need to index from the end on BE.
  n ^= elts_per_reg - 1;
#endif
  if (is_write)
    log_elt_write_if_needed(vReg);

  T *regStart = (T*)((char*)reg_file + vReg * (VLEN >> 3));
  return regStart[n];
}

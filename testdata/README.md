# RISC-V ISA Tests

This directory contains compiled RISC-V ISA compliance tests.

## Source

Tests are from the official RISC-V test suite, included as a git submodule:
- Repository: https://github.com/riscv-software-src/riscv-tests
- Location: `external/riscv-tests/`

## Directory Structure

- `isa/` - Compiled ISA test ELF binaries (physical memory mode)
  - `rv64ui-p-*` - RV64I user-level integer tests (54 tests)
  - `rv64um-p-*` - RV64M multiply/divide tests (13 tests)
  - `rv64ua-p-*` - RV64A atomic tests (19 tests)
  - `rv64uf-p-*` - RV64F single-precision floating-point tests (11 tests)
  - `rv64ud-p-*` - RV64D double-precision floating-point tests (12 tests)
  - `rv64uc-p-*` - RV64C compressed instruction tests (1 test)

## Building

Tests are built using the riscv64-linux-gnu toolchain (available on Fedora):

```bash
# Install toolchain (Fedora)
sudo dnf install gcc-riscv64-linux-gnu binutils-riscv64-linux-gnu

# Initialize submodule
git submodule update --init --recursive external/riscv-tests

# Build tests
cd external/riscv-tests
mkdir -p build && cd build
make -f ../isa/Makefile src_dir=../isa XLEN=64 RISCV_PREFIX=riscv64-linux-gnu- rv64ui rv64um rv64ua

# Build floating point tests individually (make target tries -v- tests which fail)
for f in ../isa/rv64uf/*.S; do
  name=$(basename $f .S)
  make -f ../isa/Makefile src_dir=../isa XLEN=64 RISCV_PREFIX=riscv64-linux-gnu- rv64uf-p-$name
done
for f in ../isa/rv64ud/*.S; do
  name=$(basename $f .S)
  make -f ../isa/Makefile src_dir=../isa XLEN=64 RISCV_PREFIX=riscv64-linux-gnu- rv64ud-p-$name
done

# Copy binaries (excluding .dump files)
cp rv64*-p-* ../../../testdata/riscv-tests/isa/
rm -f ../../../testdata/riscv-tests/isa/*.dump
```

## Test Format

Each test:
1. Runs in machine mode with physical addressing (loaded at 0x80000000)
2. Writes to `tohost` symbol on completion:
   - Value 1 = pass
   - Other non-zero = fail (encodes test number that failed)
3. Uses HTIF (Host Target Interface) protocol for communication

# AES-128 Full Encryption (10 Rounds)
# a0: Pointer to 16-byte plaintext (input) and ciphertext (output)
# a1: Pointer to 176-byte expanded round keys (RK0-RK10)

.section .text
.globl aes128_encrypt_full

aes128_encrypt_full:
    # Load Plaintext into state registers
    ld      t0, 0(a0)       # t0 = State_Lo (Bytes 0-7)
    ld      t1, 8(a0)       # t1 = State_Hi (Bytes 8-15)

    # --- Round 0: Initial AddRoundKey ---
    ld      t2, 0(a1)       # Load RK0_Lo
    ld      t3, 8(a1)       # Load RK0_Hi
    xor     t0, t0, t2      # State_Lo ^= RK0_Lo
    xor     t1, t1, t3      # State_Hi ^= RK0_Hi

    # --- Rounds 1-9: Middle Rounds ---
    # Each round uses 'aes64esm' (SubBytes, ShiftRows, MixColumns)
    # followed by an XOR with the next round key.

    # Round 1
    ld      t2, 16(a1)      # Load RK1_Lo
    ld      t3, 24(a1)      # Load RK1_Hi
    aes64esm t0, t0, t1     # Apply middle transform to Low half
    aes64esm t1, t1, t0     # Apply middle transform to High half
    xor     t0, t0, t2      # XOR RK1
    xor     t1, t1, t3

    # Round 2
    ld      t2, 32(a1)
    ld      t3, 40(a1)
    aes64esm t0, t0, t1
    aes64esm t1, t1, t0
    xor     t0, t0, t2
    xor     t1, t1, t3

    # Round 3
    ld      t2, 48(a1)
    ld      t3, 56(a1)
    aes64esm t0, t0, t1
    aes64esm t1, t1, t0
    xor     t0, t0, t2
    xor     t1, t1, t3

    # Round 4
    ld      t2, 64(a1)
    ld      t3, 72(a1)
    aes64esm t0, t0, t1
    aes64esm t1, t1, t0
    xor     t0, t0, t2
    xor     t1, t1, t3

    # Round 5
    ld      t2, 80(a1)
    ld      t3, 88(a1)
    aes64esm t0, t0, t1
    aes64esm t1, t1, t0
    xor     t0, t0, t2
    xor     t1, t1, t3

    # Round 6
    ld      t2, 96(a1)
    ld      t3, 104(a1)
    aes64esm t0, t0, t1
    aes64esm t1, t1, t0
    xor     t0, t0, t2
    xor     t1, t1, t3

    # Round 7
    ld      t2, 112(a1)
    ld      t3, 120(a1)
    aes64esm t0, t0, t1
    aes64esm t1, t1, t0
    xor     t0, t0, t2
    xor     t1, t1, t3

    # Round 8
    ld      t2, 128(a1)
    ld      t3, 136(a1)
    aes64esm t0, t0, t1
    aes64esm t1, t1, t0
    xor     t0, t0, t2
    xor     t1, t1, t3

    # Round 9
    ld      t2, 144(a1)
    ld      t3, 152(a1)
    aes64esm t0, t0, t1
    aes64esm t1, t1, t0
    xor     t0, t0, t2
    xor     t1, t1, t3

    # --- Round 10: Final Round ---
    # Uses 'aes64es' which omits the MixColumns step
    ld      t2, 160(a1)      # Load RK10_Lo
    ld      t3, 168(a1)      # Load RK10_Hi
    aes64es  t0, t0, t1      # Final round transform Low
    aes64es  t1, t1, t0      # Final round transform High
    xor     t0, t0, t2       # XOR RK10
    xor     t1, t1, t3

    # Store Ciphertext back to memory
    sd      t0, 0(a0)
    sd      t1, 8(a0)

    ret

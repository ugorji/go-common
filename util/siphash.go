package util

import "encoding/binary"

// SipHash returns the 64-bit SipHash-2-4 of the given byte slice.
// SipHash is a collision resistant, random and secure hash function.
//
// Algorithm adapted from
//   https://131002.net/siphash/siphash24.c
//   https://131002.net/siphash/siphash.pdf
func SipHash24(k0, k1 uint64, p []byte) uint64 {
	const blocksize = 8

	// initialization
	lenp := uint64(len(p))
	v0 := k0 ^ 0x736f6d6570736575
	v1 := k1 ^ 0x646f72616e646f6d
	v2 := k0 ^ 0x6c7967656e657261
	v3 := k1 ^ 0x7465646279746573

	// round func
	round := func() {
		v0 += v1
		v1 = v1<<13 | v1>>(64-13)
		v1 ^= v0
		v0 = v0<<32 | v0>>(64-32)

		v2 += v3
		v3 = v3<<16 | v3>>(64-16)
		v3 ^= v2

		v0 += v3
		v3 = v3<<21 | v3>>(64-21)
		v3 ^= v0

		v2 += v1
		v1 = v1<<17 | v1>>(64-17)
		v1 ^= v2
		v2 = v2<<32 | v2>>(64-32)
	}

	// compression
	for {
		var m uint64
		stop := len(p) < blocksize
		if stop {
			var x [8]byte
			copy(x[:], p)
			m = lenp<<56 | binary.LittleEndian.Uint64(x[:])
		} else {
			m = binary.LittleEndian.Uint64(p)
		}
		v3 ^= m

		round()
		round()

		v0 ^= m
		if stop {
			break
		}
		p = p[blocksize:]
	}

	// finalization
	v2 ^= 0xff

	round()
	round()
	round()
	round()

	// return
	return v0 ^ v1 ^ v2 ^ v3
}

package bits

func HalfFloatToFloatBits(h uint16) (f uint32) {
	// retrofitted from:
	// - OGRE (Object-Oriented Graphics Rendering Engine)
	//   function: halfToFloatI https://www.ogre3d.org/docs/api/1.9/_ogre_bitwise_8h_source.html
	// - http://www.java2s.com/example/java-utility-method/float-to/floattohalf-float-f-fae00.html

	s := uint32((h >> 15) & 0x01)
	m := uint32(h & 0x03ff)
	e := int32((h >> 10) & 0x1f)

	if e == 0 {
		if m == 0 { // plus or minus 0
			return s << 31
		}
		// Denormalized number -- renormalize it
		for (m & 0x0400) == 0 {
			m <<= 1
			e -= 1
		}
		e += 1
		m &= ^uint32(0x0400)
	} else if e == 31 {
		if m == 0 { // Inf
			return (s << 31) | 0x7f800000
		}
		return (s << 31) | 0x7f800000 | (m << 13) // NaN
	}
	e = e + (127 - 15)
	m = m << 13
	return (s << 31) | (uint32(e) << 23) | m
}

func FloatToHalfFloatBits(i uint32) (h uint16) {
	s := (i >> 16) & 0x8000
	e := int32(((i >> 23) & 0xff) - (127 - 15))
	m := i & 0x7fffff

	var h32 uint32

	if e <= 0 {
		if e < -10 { // zero
			h32 = s // track -0 vs +0
		} else {
			m = (m | 0x800000) >> uint32(1-e)
			h32 = s | (m >> 13)
		}
	} else if e == 0xff-(127-15) {
		if m == 0 { // Inf
			h32 = s | 0x7c00
		} else { // NAN
			m >>= 13
			var me uint32
			if m == 0 {
				me = 1
			}
			h32 = s | 0x7c00 | m | me
		}
	} else {
		if e > 30 { // Overflow
			h32 = s | 0x7c00
		} else {
			h32 = s | (uint32(e) << 10) | (m >> 13)
		}
	}
	h = uint16(h32)
	return
}

package bits

import (
	"math"
	"testing"
)

// TestHalfFloat will test all values for floats that fit within uint16,
// and ensure we can do an exact round-trip.
func TestHalfFloat(t *testing.T) {
	type c struct {
		i   uint16
		u16 uint16
		u32 uint32
	}
	var fail []c

	for i := uint16(0); i < 0xFFFF; i++ {
		if i != 0x8000 {
			continue
		}
		u32 := HalfFloatToFloatBits(i)
		u16 := FloatToHalfFloatBits(u32)
		if u16 != i {
			fail = append(fail, c{i, u16, u32})
		}
	}

	if len(fail) > 0 {
		t.Logf("HalfFloat: %v error(s)", len(fail))
		for _, v := range fail {
			t.Logf("Expecting: 0x%x, got: 0x%x (float: %v) -> 0x%x", v.i, v.u32, math.Float32frombits(v.u32), v.u16)
		}
		t.FailNow()
	}
}

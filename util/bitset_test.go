package util

import "testing"

var testBitsetTable = []struct {
	Max   int
	Set   []int
	Clear []int
	IsSet []int
}{
	{10, []int{0, 1, 5, 6, 7, 9}, []int{3, 4, 5, 6}, []int{0, 1, 7, 9}},
}

func TestBitset(t *testing.T) {
	for _, tb := range testBitsetTable {
		t.Logf("Testing: %v", tb)
		b := new(Bitset)
		for _, i := range tb.Set {
			b.Set(i)
		}
		for _, i := range tb.Clear {
			b.Clear(i)
		}
		j := 0
		for i := 0; i < tb.Max; i++ {
			bv := b.IsSet(i)
			if i == tb.IsSet[j] {
				if bv {
					j++
				} else {
					t.Logf("Expecting: %v to be set", i)
					t.FailNow()
				}
			} else if bv {
				t.Logf("Expecting: %v to not be set", i)
				t.FailNow()
			}
		}
	}
}

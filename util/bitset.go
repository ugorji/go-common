package util

/*
Bitset is used to manage a set of bits.
*/
type Bitset []byte 

func (b *Bitset) Set(i int) {
	q := i / 8
	if len(*b) <= q {
		bits2 := make([]byte, q+1)
		copy(bits2, *b)
		*b = bits2
	}
	(*b)[q] |= 1 << uint8(i % 8)
	//Logf("Set: %v", *b)
}

func (b *Bitset) Clear(i int) {
	q := i / 8
	if len(*b) <= q {
		return
	}
	(*b)[q] &^= 1 << uint8(i % 8)
	//Logf("Clear: %v", *b)
}

func (b *Bitset) IsSet(i int) bool {
	q := i / 8
	if len(*b) <= q {
		return false
	}
	return (*b)[q] & (1 << uint8(i % 8)) != 0
}


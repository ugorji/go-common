package bits

// Set manages an arbitrary set of bits.
type Set []byte

func (b *Set) Set(i int) {
	q := i / 8
	if len(*b) <= q {
		bits2 := make([]byte, q+1)
		copy(bits2, *b)
		*b = bits2
	}
	(*b)[q] |= 1 << uint8(i%8)
}

func (b *Set) Clear(i int) {
	q := i / 8
	if len(*b) <= q {
		return
	}
	(*b)[q] &^= 1 << uint8(i%8)
}

func (b *Set) Is(i int) bool {
	q := i / 8
	if len(*b) <= q {
		return false
	}
	return (*b)[q]&(1<<uint8(i%8)) != 0
}

// Set256 manages a set of 256 bits.
type Set256 [8 * 4]byte

func (x *Set256) Is(pos byte) bool {
	return x[pos>>3]&(1<<(pos&7)) != 0
}

func (x *Set256) Set(pos byte) {
	x[pos>>3] |= (1 << (pos & 7))
}

func (x *Set256) Clear(pos byte) {
	x[pos>>3] &^= (1 << (pos & 7))
}

// Set128 manages a set of 128 bits.
//
// A panic occurs if you try to
// Set, Clear or Check outside range 0-127.
type Set128 [8 * 2]byte

func (x *Set128) Is(pos byte) bool {
	return x[pos>>3]&(1<<(pos&7)) != 0
}

func (x *Set128) Set(pos byte) {
	x[pos>>3] |= (1 << (pos & 7))
}

func (x *Set128) Clear(pos byte) {
	x[pos>>3] &^= (1 << (pos & 7))
}

// Set64 manages a set of 64 bits.
//
// A panic occurs if you try to
// Set, Clear or Check outside range 0-63.
type Set64 [8 * 1]byte

func (x *Set64) Is(pos byte) bool {
	return x[pos>>3]&(1<<(pos&7)) != 0
}

func (x *Set64) Set(pos byte) {
	x[pos>>3] |= (1 << (pos & 7))
}

func (x *Set64) Clear(pos byte) {
	x[pos>>3] &^= (1 << (pos & 7))
}

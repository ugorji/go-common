package reflectutil

// MaxArrayLen is the size of uint, which determines
// the maximum length of any array.
const MaxArrayLen = 1<<((32<<(^uint(0)>>63))-1) - 1

// GrowCap will return a new capacity for a slice, given the following:
//   - oldCap: current capacity
//   - unit: in-memory size of an element
//   - num: number of elements to add
func GrowCap(oldCap, unit, num uint) (newCap uint) {
	// appendslice logic (if cap < 1024, *2, else *1.25):
	//   leads to many copy calls, especially when copying bytes.
	//   bytes.Buffer model (2*cap + n): much better for bytes.
	// smarter way is to take the byte-size of the appended element(type) into account

	// maintain 1 thresholds:
	// t1: if cap <= t1, newcap = 2x
	//     else          newcap = 1.5x
	//
	// t1 is always >= 1024.
	// This means that, if unit size >= 16, then always do 2x or 1.5x
	//
	// With this, appending for bytes increase by:
	//    100% up to 4K
	//     50% beyond that

	// unit can be 0 e.g. for struct{}{}; handle that appropriately
	maxCap := num + (oldCap * 3 / 2)
	if unit == 0 || maxCap > MaxArrayLen || maxCap < oldCap { // handle wraparound, etc
		return MaxArrayLen
	}

	const baseThreshold = 1024
	const cacheLineSize = 64

	var t1 uint = baseThreshold // default thresholds for large values
	if unit <= 4 {
		t1 = 8 * baseThreshold
	} else if unit <= 16 {
		t1 = 2 * baseThreshold
	}

	if oldCap == 0 {
		newCap = 2 + num
	} else if oldCap <= t1 { // [0,t1]
		newCap = num + oldCap + oldCap // num+(oldCap*2)
	} else { // (t1,infinity]
		newCap = maxCap
	}

	// ensure newCap takes multiples of a cache line (size is a multiple of cacheLineSize).
	// newcap*unit need not be divisible by the lowest common multiple of unit and cachelinesize.
	t1 = newCap * unit
	if t2 := t1 % cacheLineSize; t2 != 0 {
		newCap = (t1 + cacheLineSize - t2) / unit
	}

	return
}

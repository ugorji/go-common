package reflectutil

// GrowCap will return a new capacity for a slice, given the following:
//   - oldCap: current capacity
//   - unit: in-memory size of an element
//   - num: number of elements to add
func GrowCap(oldCap, unit, num int) (newCap int) {
	// appendslice logic (if cap < 1024, *2, else *1.25):
	//   leads to many copy calls, especially when copying bytes.
	//   bytes.Buffer model (2*cap + n): much better for bytes.
	// smarter way is to take the byte-size of the appended element(type) into account

	// maintain 1 thresholds:
	// t1: if cap <= t1, newcap = 2x
	//     else          newcap = 1.5x
	//
	// t1 is always >= 1024.
	// This means that, if unit size >= 16, then always do 2x or 1.5x (ie t1, t2, t3 are all same)
	//
	// With this, appending for bytes increase by:
	//    100% up to 4K
	//     50% beyond that

	// unit can be 0 e.g. for struct{}{}; handle that appropriately
	if unit <= 0 {
		if uint64(^uint(0)) == ^uint64(0) { // 64-bit
			var maxInt64 uint64 = 1<<63 - 1 // prevent failure with overflow int on 32-bit (386)
			return int(maxInt64)            // math.MaxInt64
		}
		return 1<<31 - 1 //  math.MaxInt32
	}

	// handle if num < 0, cap=0, etc.

	var t1 int = 1024 // default thresholds for large values
	if unit <= 4 {
		t1 = 8 * 1024
	} else if unit <= 16 {
		t1 = 2 * 1024
	}

	if oldCap <= 0 {
		newCap = 2
	} else if oldCap <= t1 { // [0,t1]
		newCap = oldCap * 2
	} else { // (t1,infinity]
		newCap = oldCap * 3 / 2
	}

	if num > 0 && newCap < num+oldCap {
		newCap = num + oldCap
	}

	// ensure newCap takes multiples of a cache line (size is a multiple of 64)
	t1 = newCap * unit
	if t2 := t1 % 64; t2 != 0 {
		t1 += 64 - t2
		newCap = t1 / unit
	}

	return
}

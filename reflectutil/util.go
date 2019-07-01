package reflectutil

// GrowCap will return a new capacity for a slice, given the following:
//   - oldCap: current capacity
//   - unit: in-memory size of an element
//   - num: number of elements to add
func GrowCap(oldCap, unit, num int) (newCap int) {
	// design this well so it can be easily inlined.
	// This means:
	//   - no constants, panic, switch, etc

	// appendslice logic (if cap < 1024, *2, else *1.25):
	//   leads to many copy calls, especially when copying bytes.
	//   bytes.Buffer model (2*cap + n): much better for bytes.
	// smarter way is to take the byte-size of the appended element(type) into account

	// maintain 3 thresholds:
	// t1: if cap <= t1, newcap = 2x
	// t2: if cap <= t2, newcap = 1.75x
	// t3: if cap <= t3, newcap = 1.5x
	//     else          newcap = 1.25x
	//
	// t1, t2, t3 >= 1024 always.
	// This means that, if unit size >= 16, then always do 2x or 1.25x (ie t1, t2, t3 are all same)
	//
	// With this, appending for bytes increase by:
	//    100% up to 4K
	//     75% up to 8K
	//     50% up to 16K
	//     25% beyond that

	// unit can be 0 e.g. for struct{}{}; handle that appropriately
	// handle if num < 0, cap=0, etc.
	var t1, t2, t3 int // thresholds
	if unit <= 1 {
		t1, t2, t3 = 4*1024, 8*1024, 16*1024
	} else if unit < 16 {
		t3 = 16 / unit * 1024
		t1 = t3 * 1 / 4
		t2 = t3 * 2 / 4
	} else {
		t1, t2, t3 = 1024, 1024, 1024
	}

	var x int // temporary variable

	// x is multiplier here.
	// x is either 5, 6, 7 or 8 (for incr of 25%, 50%, 75% or 100% respectively)
	if oldCap <= t1 { // [0,t1]
		x = 8
	} else if oldCap > t3 { // (t3,infinity]
		x = 5
	} else if oldCap <= t2 { // (t1,t2]
		x = 7
	} else { // (t2,t3]
		x = 6
	}
	newCap = x * oldCap / 4

	if num > 0 {
		newCap += num
	}
	if newCap <= oldCap {
		newCap = oldCap + 1
	}

	// ensure newCap is a multiple of 64 (if it is > 64)
	// we want this inlined, so DONT use a const declaration.
	// TODO: After inl handles ODCLCONST, then use a const for 65
	// const growToCapDiv = 64 // return caps as a multiple of 8 64-bit words
	if newCap > 64 {
		if x = newCap % 64; x != 0 {
			x = newCap / 64
			newCap = 64 * (x + 1)
		}
	} else {
		if x = newCap % 16; x != 0 {
			x = newCap / 16
			newCap = 16 * (x + 1)
		}
	}
	return
}

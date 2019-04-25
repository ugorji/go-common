package bits

func PruneSignExt(v []byte, pos bool) (n int) {
	if len(v) < 2 {
	} else if pos && v[0] == 0 {
		for ; v[n] == 0 && n+1 < len(v) && (v[n+1]&(1<<7) == 0); n++ {
		}
	} else if !pos && v[0] == 0xff {
		for ; v[n] == 0xff && n+1 < len(v) && (v[n+1]&(1<<7) != 0); n++ {
		}
	}
	return
}

func PruneLeading(v []byte, pruneVal byte) (n int) {
	if len(v) < 2 {
		return
	}
	for ; v[n] == pruneVal && n+1 < len(v) && (v[n+1] == pruneVal); n++ {
	}
	return
}

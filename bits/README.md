# go-common/bits

This repository contains the `go-common/bits` library.

To install:

```
go get github.com/ugorji/go-common/bits
```

# Package Documentation


Package bits enables dealing with bit sets.

## Exported Package API

```go
func FloatToHalfFloatBits(i uint32) (h uint16)
func HalfFloatToFloatBits(h uint16) (f uint32)
func PruneLeading(v []byte, pruneVal byte) (n int)
func PruneSignExt(v []byte, pos bool) (n int)
type Set []byte
type Set128 [8 * 2]byte
type Set256 [8 * 4]byte
type Set64 [8 * 1]byte
```

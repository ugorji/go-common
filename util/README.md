# go-common/util

This repository contains the `go-common/util` library (or command).

To install:

```
go get github.com/ugorji/go-common/util
```

# Package Documentation


Utilities: containers, types, etc.

It includes utility functions and utility types which across the codebase.

The utility types include:

    - tree (Int64, interface{})
    - bitset
    - combination generator
    - cron definition and support
    - buffered byte reader which does not include copying (as compared to bytes.Buffer)
    - lock set
    - safe store
    - virtual file system

## Exported Package API

```go
const InterpolatePrefix string = "${" ...
var ErrNotDir = errors.New("not a directory")
var LogFn func(format string, params ...interface{})
func AbsPath(path string) (abspath string, err error)
func AllFieldNames(rt reflect.Type, exported bool, names map[string]reflect.StructField) map[string]reflect.StructField
func ApproxDataSize(rv reflect.Value) (sum int)
func ChkDir(dir string) (exists, isDir bool, err error)
func Coerce(val interface{}, typ interface{}) (interface{}, error)
func CoerceRV(rval reflect.Value, rtyp reflect.Type) (reflect.Value, error)
func CopyFile(dest, src string, createDirs bool) (err error)
func CopyFileToWriter(dest io.Writer, src string) (err error)
func DebugLineInfo(calldepth uint8, unsetVal string) (subsystem, file string, line int, func0 string)
func Debugf(format string, args ...interface{})
func DeepEqual(v1, v2 interface{}, strict bool) (err error)
func ExpandSliceValue(s reflect.Value, num int) reflect.Value
func FindFreeLocalPort() (port int, err error)
func GrowCap(oldCap, unit, num int) (newCap int)
func HalfFloatToFloatBits(yy uint16) (d uint32)
func ImplementsInterface(typ, iTyp reflect.Type) (success bool, indir int8)
func Indir(rv reflect.Value, finalTyp reflect.Type, maxDepth int) reflect.Value
func IndirIntf(v interface{}, finalTyp reflect.Type, maxDepth int) interface{}
func Interpolate(s string, vars map[string]interface{}) string
func IsEmptyValue(v reflect.Value, deref, checkStruct bool) bool
func MkDir(dir string) (err error)
func OpenInApplication(uri string) error
func ParseRegexTemplate(s string) (re *regexp.Regexp, clean string, takeys []string, err error)
func PathMatchesStaticFile(path string) (isMatch bool)
func PruneLeading(v []byte, pruneVal byte) (n int)
func PruneSignExt(v []byte, pos bool) (n int)
func SipHash24(k0, k1 uint64, p []byte) uint64
func Stack(bs []byte, all bool) []byte
func SymlinkTarget(fi os.FileInfo, fpath string) (fpath2 string, changed bool, err error)
func ToGeneric(in interface{}) (out interface{})
func UUID(xlen int) (string, error)
func UUID2() (uuid []byte, err error)
func ValuePrintf(v interface{}) string
func ValueToErr(panicVal interface{}, err *error)
func WriteFile(dest string, contents []byte, createDirs bool) (err error)
type Bitset []byte
type BufReader struct{ ... }
    func NewBufReader(r io.Reader, b []byte) (br *BufReader)
type BufWriter struct{ ... }
    func NewBufWriter(w io.Writer, b []byte) (bw *BufWriter)
type ValuePrintfer struct{ ... }
```

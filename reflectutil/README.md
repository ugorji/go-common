# go-common/reflectutil

This repository contains the `go-common/reflectutil` library (or command).

To install:

```
go get github.com/ugorji/go-common/reflectutil
```

# Package Documentation


Package reflectutil provides reflection utilities.

## Exported Package API

```go
func AllFieldNames(rt reflect.Type, exported bool, names map[string]reflect.StructField) map[string]reflect.StructField
func ApproxDataSize(rv reflect.Value) (sum int)
func Coerce(val interface{}, typ interface{}) (interface{}, error)
func CoerceRV(rval reflect.Value, rtyp reflect.Type) (reflect.Value, error)
func DeepEqual(v1, v2 interface{}, strict bool) (err error)
func ExpandSliceValue(s reflect.Value, num int) reflect.Value
func GrowCap(oldCap, unit, num int) (newCap int)
func ImplementsInterface(typ, iTyp reflect.Type) (success bool, indir int8)
func Indir(rv reflect.Value, finalTyp reflect.Type, maxDepth int) reflect.Value
func IndirIntf(v interface{}, finalTyp reflect.Type, maxDepth int) interface{}
func IsEmptyValue(v reflect.Value, deref, checkStruct bool) bool
func ToGeneric(in interface{}) (out interface{})
```

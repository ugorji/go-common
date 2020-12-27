# go-common/testutil

This repository contains the `go-common/testutil` library.

To install:

```
go get github.com/ugorji/go-common/testutil
```

# Package Documentation


Package testutil provides testing utilities.

## Exported Package API

```go
func CheckEqual(t *testing.T, v1 interface{}, v2 interface{}, desc string) (err error)
func CheckErr(t *testing.T, err error)
func Fail(t *testing.T)
func Log(x interface{}, format string, args ...interface{})
```

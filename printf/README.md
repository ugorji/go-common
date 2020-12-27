# go-common/printf

This repository contains the `go-common/printf` library.

To install:

```
go get github.com/ugorji/go-common/printf
```

# Package Documentation


Package printf provides utilities for formatted printing.

## Exported Package API

```go
func Debugf(format string, args ...interface{})
func ValuePrintf(v interface{}) string
type ValuePrintfer struct{ ... }
```

# go-common/runtimeutil

This repository contains the `go-common/runtimeutil` library (or command).

To install:

```
go get github.com/ugorji/go-common/runtimeutil
```

# Package Documentation


Package runtimeutil provides runtime utilities.

Some functions are only available if debugging is enabled. Enable debugging
by adding "ugorji=1" to the GODEBUG environmental variable.

## Exported Package API

```go
func Debug() bool
func FileLine(calldepth uint8) (func0, file string, line int)
func FuncFileLine(calldepth uint8) (func0, file string, line int)
func P(pattern string, args ...interface{})
func PkgFuncFileLine(calldepth uint8) (subsystem, func0, file string, line int)
func Stack(bs []byte, all bool) []byte
```

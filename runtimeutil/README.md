# go-common/runtimeutil

This repository contains the `go-common/runtimeutil` library (or command).

To install:

```
go get github.com/ugorji/go-common/runtimeutil
```

# Package Documentation


Package runtimeutil provides runtime utilities.

## Exported Package API

```go
func DebugLineInfo(calldepth uint8, unsetVal string) (subsystem, file string, line int, func0 string)
func Stack(bs []byte, all bool) []byte
```

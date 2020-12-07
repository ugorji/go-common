# go-common/flagutil

This repository contains the `go-common/flagutil` library (or command).

To install:

```
go get github.com/ugorji/go-common/flagutil
```

# Package Documentation


## Exported Package API

```go
type BoolFlagValue struct{ ... }
type RegexpFlagValue regexp.Regexp
type SetStringFlagValue struct{ ... }
type StringsFlagValue []string
type StringsNoDupFlagValue []string
```

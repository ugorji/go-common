# go-common/combinationgen

This repository contains the `go-common/combinationgen` library (or command).

To install:

```
go get github.com/ugorji/go-common/combinationgen
```

# Package Documentation


## Exported Package API

```go
type T struct{ ... }
    func New(vprops []interface{}, combo [][]interface{}) (cg *T, err error)
```
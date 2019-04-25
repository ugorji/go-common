# go-common/pool

This repository contains the `go-common/pool` library (or command).

To install:

```
go get github.com/ugorji/go-common/pool
```

# Package Documentation


Package pool manages a pool of resources.

## Exported Package API

```go
func Must(v interface{}, err error) interface{}
type Action uint8
    const GET Action = iota ...
type Fn func(v interface{}, a Action, currentLen int) (interface{}, error)
type T struct{ ... }
    func New(fn Fn, load, capacity int) (t *T, err error)
```

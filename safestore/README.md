# go-common/safestore

This repository contains the `go-common/safestore` library.

To install:

```
go get github.com/ugorji/go-common/safestore
```

# Package Documentation


Package safestore provides a possibly threadsafe key-value cache.

## Exported Package API

```go
type I interface{ ... }
type Item struct{ ... }
type T struct{ ... }
    func New(useLock bool) *T
```

# go-common/ioutil

This repository contains the `go-common/ioutil` library.

To install:

```
go get github.com/ugorji/go-common/ioutil
```

# Package Documentation


Package ioutil provides input/output utilities.

## Exported Package API

```go
type BufReader struct{ ... }
    func NewBufReader(r io.Reader, b []byte) (br *BufReader)
type BufWriter struct{ ... }
    func NewBufWriter(w io.Writer, b []byte) (bw *BufWriter)
```

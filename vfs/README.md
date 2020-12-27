# go-common/vfs

This repository contains the `go-common/vfs` library.

To install:

```
go get github.com/ugorji/go-common/vfs
```

# Package Documentation


Package vfs implements a virtual file system.

## Exported Package API

```go
var ErrInvalid = os.ErrInvalid
var ErrReadNotImmutable = errors.New("vfs: cannot read immutable contents")
type FS interface{ ... }
type File interface{ ... }
type FileInfo interface{ ... }
type MemFS struct{ ... }
type MemFile struct{ ... }
type OsFS struct{ ... }
    func NewOsFS(fpath string) (z *OsFS, err error)
type Vfs struct{ ... }
type WithReadDir interface{ ... }
type WithReadImmutable interface{ ... }
type ZipFS struct{ ... }
    func NewZipFS(r *zip.ReadCloser) (z *ZipFS)
```

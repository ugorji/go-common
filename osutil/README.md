# go-common/osutil

This repository contains the `go-common/osutil` library (or command).

To install:

```
go get github.com/ugorji/go-common/osutil
```

# Package Documentation


Package osutil provides utilities functions for the operating system.

## Exported Package API

```go
func AbsPath(path string) (abspath string, err error)
func ChkDir(dir string) (exists, isDir bool, err error)
func CopyFile(dest, src string, createDirs bool) (err error)
func CopyFileToWriter(dest io.Writer, src string) (err error)
func IsTerminal(fd int) bool
func MkDir(dir string) (err error)
func OpenInApplication(uri string) error
func SymlinkTarget(fi os.FileInfo, fpath string) (fpath2 string, changed bool, err error)
func WriteFile(dest string, contents []byte, createDirs bool) (err error)
```

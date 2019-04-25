# go-common/regexputil

This repository contains the `go-common/regexputil` library (or command).

To install:

```
go get github.com/ugorji/go-common/regexputil
```

# Package Documentation


Package regexputil provides utility functions for regexp.

## Exported Package API

```go
func Interpolate(s string, vars map[string]interface{}) string
func ParseRegexTemplate(s string) (re *regexp.Regexp, clean string, takeys []string, err error)
func PathMatchesStaticFile(path string) (isMatch bool)
```

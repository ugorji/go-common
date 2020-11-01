package vfs

import (
	"archive/zip"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// var zipRootFilesRe = regexp.MustCompile(`^[^\\/]+[\\/]?$`)

// ZipFS is a FileSystem that serves files out of a zip file
type ZipFS struct {
	*zip.ReadCloser
	m map[string]*zipFileEntry
}

// zipFileEntry is an entry in a zip file
type zipFileEntry struct {
	*zip.File
	cleanName string
	z         *ZipFS
}

type zipFile struct {
	*zipFileEntry
	io.ReadCloser
}

func NewZipFS(r *zip.ReadCloser) (z *ZipFS) {
	z = &ZipFS{r, make(map[string]*zipFileEntry, len(r.File))}
	for _, f := range r.File {
		zf := &zipFileEntry{f, path.Clean(f.Name), z}
		z.m[zf.cleanName] = zf
	}
	return
}

func (x *ZipFS) Matches(matchRe, notMatchRe *regexp.Regexp, includeDirs bool) (names []string, err error) {
	for k, v := range x.m {
		if !includeDirs && v.FileInfo().IsDir() {
			continue
		}
		if (matchRe == nil || matchRe.MatchString(k)) &&
			(notMatchRe == nil || !notMatchRe.MatchString(k)) {
			names = append(names, k)
		}
	}
	return
}

func (x *ZipFS) matchesInfo(basepath, pattern string, n int) (infos []FileInfo, err error) {
	var matches bool
	var fi FileInfo
	for k, v := range x.m {
		if basepath == "" || strings.HasPrefix(k, basepath) {
			k = k[len(basepath):]
			matches, err = filepath.Match(k, pattern)
			if err != nil {
				return
			}
			if matches {
				fi, err = v.Stat()
				if err != nil {
					return
				}
				infos = append(infos, fi)
				if n > 0 && len(infos) == n {
					return
				}
			}
		}
	}
	return
}

func (x *ZipFS) RootFiles() (infos []FileInfo, err error) {
	return x.matchesInfo("", "*", -1)
}

func (x *ZipFS) Open(name string) (f File, err error) {
	z, ok := x.m[path.Clean(name)]
	if !ok {
		return nil, ErrInvalid
	}
	rc, err := z.Open()
	if err != nil {
		return
	}
	return &zipFile{z, rc}, nil
}

func (x *zipFileEntry) ReadDir(n int) (infos []FileInfo, err error) {
	return x.z.matchesInfo(x.cleanName, "*", -1)
}

func (x *zipFileEntry) Stat() (fi FileInfo, err error) {
	return x.FileInfo(), nil
}

var _, _, _ = FS((*ZipFS)(nil)), File((*zipFile)(nil)), FileInfo((os.FileInfo)(nil))

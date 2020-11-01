package vfs

import (
	"os"
	"path/filepath"
	"regexp"
)

// OsFS is a FileSystem that serves files out of a os file
type OsFS struct {
	f *os.File
}

// osFile is an entry in a os file
type osFile struct {
	*os.File
}

func NewOsFS(fpath string) (z *OsFS, err error) {
	f, err := os.Open(filepath.Clean(fpath))
	if err != nil {
		return
	}
	z = &OsFS{f}
	return
}

func (x *OsFS) Matches(matchRe, notMatchRe *regexp.Regexp, includeDirs bool) (names []string, err error) {
	base := x.f.Name()
	m := make(map[string]struct{})

	walkfn := func(path string, info os.FileInfo, err error) error {
		if !includeDirs && info.IsDir() {
			return nil
		}
		if _, ok := m[path]; ok {
			return nil
		}
		m[path] = struct{}{}
		if len(base) == len(path) {
			return nil
		}
		path2 := path[len(base)+1:]
		if (matchRe == nil || matchRe.MatchString(path2)) &&
			(notMatchRe == nil || !notMatchRe.MatchString(path2)) {
			names = append(names, path2)
		}
		return nil
	}

	err = filepath.Walk(base, walkfn)
	return
}

func osList(f *os.File, n int) (infos []FileInfo, err error) {
	s, err := f.Readdir(n)
	if err != nil || s == nil {
		return
	}
	infos = make([]FileInfo, len(s))
	for i := range s {
		infos[i] = s[i]
	}
	return
}

func (x *OsFS) RootFiles() (infos []FileInfo, err error) {
	return osList(x.f, -1)
}

func (x *OsFS) Open(name string) (f File, err error) {
	zf, err := os.Open(filepath.Join(x.f.Name(), name))
	if err != nil {
		return
	}
	return &osFile{zf}, nil
}

func (x *OsFS) Close() error {
	return x.f.Close()
}

func (x *osFile) ReadDir(n int) (infos []FileInfo, err error) {
	return osList(x.File, n)
}

func (x *osFile) Stat() (info FileInfo, err error) {
	info2, err := x.File.Stat()
	return info2, err
}

var _, _, _ = FS((*OsFS)(nil)), File((*osFile)(nil)), FileInfo((os.FileInfo)(nil))

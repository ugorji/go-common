package vfs

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type MemFS struct {
	files  map[string]*MemFile
	sealed bool
}

func (x *MemFS) Close() error { return nil }

func (x *MemFS) Open(name string) (f File, err error) {
	mf, ok := x.files[path.Clean(name)]
	if !ok {
		return nil, ErrInvalid
	}
	f = &memFileReadCloser{mf, strings.NewReader(mf.content)}
	return
}

func (x *MemFS) RootFiles() (infos []FileInfo, err error) {
	return x.matchesInfo("", "*", -1)
}

func (x *MemFS) matchesInfo(basepath, pattern string, n int) (infos []FileInfo, err error) {
	var matches bool
	for k, v := range x.files {
		if basepath == "" || strings.HasPrefix(k, basepath) {
			k = k[len(basepath):]
			matches, err = filepath.Match(k, pattern)
			if err != nil {
				return
			}
			if matches {
				infos = append(infos, v)
				if n > 0 && len(infos) == n {
					return
				}
			}
		}
	}
	return
}

func (x *MemFS) Matches(matchRe, notMatchRe *regexp.Regexp, includeDirs bool) (names []string, err error) {
	for k, v := range x.files {
		if !includeDirs && v.IsDir() {
			continue
		}
		if (matchRe == nil || matchRe.MatchString(k)) &&
			(notMatchRe == nil || !notMatchRe.MatchString(k)) {
			names = append(names, k)
		}
	}
	return
}

func (x *MemFS) AddFile(parent *MemFile, name string, size int64, modTime time.Time, content string) (m *MemFile) {
	if x.sealed {
		x.sealed = false
	}
	name = path.Clean(name)
	m = &MemFile{
		content: content,
		parent:  parent,
		name:    name,
		size:    size,
		modTime: modTime,
		fs:      x,
	}
	x.files[name] = m
	return
}

func (x *MemFS) GetFile(name string) *MemFile {
	return x.files[path.Clean(name)]
}

// Seal says that we are done modifying the MemFS.
//
// The MemFS can then cache some internal metrics, to optimize things like MemFile.IsDir, etc.
//
// If modification is done to the MemFS after this, the optimizations are removed.
func (x *MemFS) Seal() {
	if x.sealed {
		return
	}
	for _, v := range x.files {
		if v.parent != nil && !v.parent.dir {
			v.parent.dir = true
		}
	}
	x.sealed = true
}

func (x *MemFS) isDir(f *MemFile) bool {
	if x.sealed {
		return f.dir
	}
	for _, v := range x.files {
		if v.parent == f {
			return true
		}
	}
	return false
}

type MemFile struct {
	name    string
	size    int64
	modTime time.Time
	content string
	r       *strings.Reader
	parent  *MemFile
	fs      *MemFS
	dir     bool
}

func (x *MemFile) Name() string                   { return x.name }
func (x *MemFile) Size() int64                    { return x.size }
func (x *MemFile) ModTime() time.Time             { return x.modTime }
func (x *MemFile) ReadImmutable() (string, error) { return x.content, nil }
func (x *MemFile) Stat() (FileInfo, error)        { return x, nil }

func (x *MemFile) IsDir() bool {
	return x.fs.isDir(x)
}

func (x *MemFile) ReadDir(n int) (infos []FileInfo, err error) {
	// find all memfiles in this filesystem that have this as their parent, and return their FileInfo
	for _, v := range x.fs.files {
		if v.parent == x {
			infos = append(infos, v)
			if n > 0 && len(infos) == n {
				return
			}
		}
	}
	if len(infos) == 0 {
		err = ErrInvalid
	}
	return
}

type memFileReadCloser struct {
	*MemFile
	r *strings.Reader
}

func (x *memFileReadCloser) Read(p []byte) (n int, err error) { return x.r.Read(p) }
func (x *memFileReadCloser) Close() error                     { return nil }

var _, _, _ = FS((*MemFS)(nil)), File((*memFileReadCloser)(nil)), FileInfo((*MemFile)(nil))

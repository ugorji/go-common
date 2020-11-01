package vfs

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/ugorji/go-common/errorutil"
)

var ErrReadNotImmutable = errors.New("vfs: cannot read immutable contents")
var ErrInvalid = os.ErrInvalid

// FileInfo holds file metadata.
//
// It is intentionally a subset of os.FileInfo, so that it can interop well with the standard lib.
type FileInfo interface {
	Name() string       // base name of the file
	Size() int64        // length in bytes for regular files; system-dependent for others
	ModTime() time.Time // modification time
	IsDir() bool        // abbreviation for Mode().IsDir()
}

// File is an open'ed entry in a file system
type File interface {
	io.ReadCloser
	Stat() (FileInfo, error)
}

// WithReadImmutable is implemented by Files
// that store their content as an uncompressed immutable string, can provide
// the ReadImmutable method for efficient use.
type WithReadImmutable interface {
	ReadImmutable() (string, error)
}

// WithReadDir is implemented by Directories
// to get a listing of the files in them.
type WithReadDir interface {
	ReadDir(n int) ([]FileInfo, error)
}

// FS defines the interface for a filesystem (e.g. directory, zip-based, in-memory).
type FS interface {
	// Open will open a file given its path.
	Open(name string) (File, error)
	// Close will discard resources in use.
	Close() error
	// Matches returns a list of paths within the FS that match a regexp (if defined), and
	// do not match a second regexp (if defined), with an option to include or ignore directories.
	Matches(match, notMatch *regexp.Regexp, includeDirs bool) (names []string, err error)
	// RootFiles defines all the top level files.
	//
	// For example, if there is a filesystem with just 2 files, there's no single root directory
	// but those 2 files are the root files.
	RootFiles() (infos []FileInfo, err error)
}

// Vfs provides a simple virtual file system.
//
// User can request a file from a sequence of directories or zip files,
// and once it is located, it is returned as a ReadCloser,
// along with some metadata (like lasModTime, etc).
type Vfs struct {
	fs []FS
}

// Add a FS to the Vfs based off the path, which is either a directory, a zip file or other file.
//
// Any zip file added is immediately opened for reading right away,
// and keept it open until Close is explicitly called.
func (vfs *Vfs) Add(failOnMissingFile bool, path string) (err error) {
	fi, err := os.Stat(path)
	if err != nil {
		if !failOnMissingFile {
			err = nil
		}
		return
	}
	var rc *zip.ReadCloser
	if !fi.IsDir() {
		rc, err = zip.OpenReader(path)
		if err == nil {
			vfs.fs = append(vfs.fs, NewZipFS(rc))
			return
		}
	}
	fs, err := NewOsFS(path)
	if err == nil {
		vfs.fs = append(vfs.fs, fs)
	}
	return
}

// Adds will call Add(...) on each path passed
func (vfs *Vfs) Adds(failOnMissingFile bool, paths ...string) (err error) {
	var em errorutil.Multi
	for _, path := range paths {
		if err = vfs.Add(failOnMissingFile, path); err != nil {
			em = append(em, err)
		}
	}
	if len(em) > 0 {
		err = em
	}
	return
}

// Close this Vfs. It is especially useful for the zip type PathInfos in here.
func (vfs *Vfs) Close() (err error) {
	var em errorutil.Multi
	for _, pi := range vfs.fs {
		if err = pi.Close(); err != nil {
			em = append(em, err)
		}
	}
	if len(em) > 0 {
		err = em
	}
	return
}

// Find a file from the Vfs, given a path. It will try each PathInfo in
// sequence until it finds the path requested.
func (vfs *Vfs) Find(path string) (f File, err error) {
	for _, pi := range vfs.fs {
		f, err = pi.Open(path)
		if err != nil || f != nil {
			return
		}
	}
	return
}

// Find all the paths in this Vfs which match the given reg
func (vfs *Vfs) Matches(matchRe, notMatchRe *regexp.Regexp, includeDirs bool) []string {
	var m = make(map[string]struct{})
	for _, pi := range vfs.fs {
		ss, err := pi.Matches(matchRe, notMatchRe, includeDirs)
		if err != nil {
			return nil
		}
		for _, s := range ss {
			m[s] = struct{}{}
		}
	}
	ss := make([]string, 0, len(m))
	for s := range m {
		ss = append(ss, s)
	}
	return ss
}

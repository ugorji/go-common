package vfs

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/ugorji/go-common/errorutil"
)

const (
	vfsDirType vfsPathType = iota + 1
	vfsZipType
)

//Information about a vfs path
type Metadata struct {
	ModTimeNs int64
}

/*
Vfs provides a simple virtual file system.

User can request a file from a sequence of directories or zip files,
and once it is located, it is returned as a ReadCloser,
along with some metadata (like lasModTime, etc).
*/
type Vfs struct {
	pathinfos []*pathInfo
}

type vfsPathType int32

type pathInfo struct {
	path string
	typ  vfsPathType
	zrc  *zip.ReadCloser
}

//structure used during walks
type vfsDirwalk struct {
	base string          //the base path (so we can ensure that it is not included in matches)
	m    map[string]bool //the paths which have been checked
	l    []string        //the actual matches
	r    *regexp.Regexp  //the regular expression to match against
}

// Add a path to the Vfs. This path can be a directory or a zip file.
// Note: If a zip file, we open it for reading right away, and keep it open
// till Close is explicitly called.
func (vfs *Vfs) Add(failOnMissingFile bool, path string) error {
	if vfs.pathinfos == nil {
		vfs.pathinfos = make([]*pathInfo, 0, 4)
	}
	fi, err := os.Stat(path)
	if err != nil {
		if failOnMissingFile {
			return err
		} else {
			return nil
		}
	}
	if fi.IsDir() {
		vfs.pathinfos = append(vfs.pathinfos, &pathInfo{path, vfsDirType, nil})
	} else if !fi.IsDir() {
		rc, err := zip.OpenReader(path)
		if err != nil {
			return err
		}
		vfs.pathinfos = append(vfs.pathinfos, &pathInfo{path, vfsZipType, rc})
	} else {
		return fmt.Errorf("The path: %v, is not a directory or a zip file", path)
	}
	return nil
}

func (vfs *Vfs) Adds(failOnMissingFile bool, paths ...string) (err error) {
	for _, path := range paths {
		if err = vfs.Add(failOnMissingFile, path); err != nil {
			return
		}
	}
	return
}

// Find a file from the Vfs, given a path. It will try each PathInfo in
// sequence until it finds the path requested.
func (vfs *Vfs) Find(path string) (io.ReadCloser, *Metadata, error) {
	for _, pi := range vfs.pathinfos {
		switch pi.typ {
		case vfsDirType:
			fp := filepath.Clean(filepath.Join(pi.path, path))
			fi, err := os.Stat(fp)
			if err == nil {
				frc, err := os.Open(fp)
				return frc, &Metadata{fi.ModTime().UnixNano()}, err
			}
		case vfsZipType:
			for _, zf := range pi.zrc.File {
				if zf.Name == path {
					zfrc, err := zf.Open()
					mtimeNs := zf.ModTime().UnixNano()
					return zfrc, &Metadata{mtimeNs}, err
				}
			}
		}
	}
	return nil, nil, fmt.Errorf("Path not found: %v", path)
}

// Find all the paths in this Vfs which match the given reg
func (vfs *Vfs) Matches(r *regexp.Regexp) []string {
	l := make([]string, 0, 4)

	dw := &vfsDirwalk{"", make(map[string]bool), l, r}

	walkfn := func(path string, info os.FileInfo, err error) error {
		if _, ok := dw.m[path]; !ok {
			dw.m[path] = true
			if len(dw.base) == len(path) {
				return nil
			}
			path2 := path[len(dw.base)+1:]
			match := dw.r.Match([]byte(path2))
			// logfn("Vfs: Visiting: %v: match: %v", path2, match)
			if match {
				dw.l = append(dw.l, path2)
			}
		}
		return nil
	}

	for _, pi := range vfs.pathinfos {
		// logfn("PATH: %v", pi.path)
		switch pi.typ {
		case vfsDirType:
			dw.base = pi.path
			filepath.Walk(pi.path, walkfn)
			dw.base, l = "", dw.l
		case vfsZipType:
			for _, zf := range pi.zrc.File {
				if r.Match([]byte(zf.Name)) {
					l = append(l, zf.Name)
				}
			}
		}
	}

	return l
}

// Close this Vfs. It is especially useful for the zip type PathInfos in here.
func (vfs *Vfs) Close() error {
	em := make(errorutil.Multi, 0, 2)
	for _, pi := range vfs.pathinfos {
		if pi.typ == vfsZipType {
			if err := pi.zrc.Close(); err != nil {
				em = append(em, err)
			}
		}
	}
	if len(em) == 0 {
		return nil
	}
	return em
}

package osutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

var errNotDir = errors.New("not a directory")

// Mkdir creates a directory.
func MkDir(dir string) (err error) {
	fi, err := os.Stat(dir)
	if fi == nil {
		if err != nil && os.IsNotExist(err) {
			err = os.Mkdir(dir, 0777)
		}
	} else {
		if !fi.Mode().IsDir() {
			err = errNotDir
		}
	}
	return
}

func ChkDir(dir string) (exists, isDir bool, err error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return
	}
	switch {
	case fi == nil:
		err = os.ErrNotExist
	case !fi.Mode().IsDir():
		exists = true
		err = errNotDir
	default:
		exists, isDir = true, true
	}
	return
}

func AbsPath(path string) (abspath string, err error) {
	if abspath, err = filepath.Abs(path); err != nil {
		return
	}
	abspath, err = filepath.EvalSymlinks(abspath)
	return
}

func CopyFile(dest, src string, createDirs bool) (err error) {
	var outfile *os.File
	if createDirs {
		if err = os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
			return
		}
	}
	if outfile, err = os.Create(dest); err != nil {
		return
	}
	defer outfile.Close()
	return CopyFileToWriter(outfile, src)
}

func CopyFileToWriter(dest io.Writer, src string) (err error) {
	var infile *os.File
	if infile, err = os.Open(src); err != nil {
		return
	}
	defer infile.Close()
	_, err = io.Copy(dest, infile)
	return
}

func WriteFile(dest string, contents []byte, createDirs bool) (err error) {
	if createDirs {
		if err = os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
			return
		}
	}
	zf, err := os.Create(dest)
	if err != nil {
		return
	}
	defer zf.Close()
	_, err = zf.Write(contents)
	return
}

const useEvalSymLink = true

func SymlinkTarget(fi os.FileInfo, fpath string) (fpath2 string, changed bool, err error) {
	// EvalSymLinks tries to remove all symlinks from the whole path
	// The logic here prefers to just ensure that the final file is not a symlink, but points to an inode directly
	if fi == nil {
		if fi, err = os.Lstat(fpath); err != nil {
			return
		}
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		fpath2 = fpath
		return
	}
	changed = true
	if useEvalSymLink {
		fpath2, err = filepath.EvalSymlinks(fpath)
		return
	}
	fpath2, err = os.Readlink(fpath)
	if err != nil {
		return
	}
	fpath2, _, err = SymlinkTarget(nil, fpath2)
	return
}

func OpenInApplication(uri string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("cmd", "/c", "start", uri).Run()
	case "darwin":
		return exec.Command("open", uri).Run()
	case "linux", "freebsd", "netbsd", "openbsd":
		return exec.Command("xdg-open", uri).Run()
	}
	return fmt.Errorf("cannot open: %s on OS: %s", uri, runtime.GOOS)
}

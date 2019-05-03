package runtimeutil

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"unicode"
)

var debug bool

func init() {
	debug = strings.Contains(os.Getenv("GODEBUG"), "ugorji=1")
}

func Debug() bool {
	return debug
}

// pkgFuncFileLine will return the line information.
//
// Examples:
//   1. func NewDriver() IN ugorji.net/ndb/driver.go
//      subsystem: ndb
//      file/line: driver.go
//      func0:     NewDriver
//   2. func (*Ldb) SvrPut IN ugorji.net/ndb/server/leveldb.go
//      subsystem: ndb/server
//      file/line: leveldb.go
//      func0:     (*Ldb).SvrPut
//   2. func (*Ldb) SvrPut IN github.com/ugorji/go-ndb/ndb/server/leveldb.go
//      subsystem: ugorji/go-ndb/ndb/server
//      file/line: leveldb.go
//      func0:     (*Ldb).SvrPut
func PkgFuncFileLine(calldepth uint8) (subsystem, func0, file string, line int) {
	return pkgFuncFileLine(calldepth+1, true, true)
}

func FuncFileLine(calldepth uint8) (func0, file string, line int) {
	_, func0, file, line = pkgFuncFileLine(calldepth+1, false, true)
	return
}

func FileLine(calldepth uint8) (func0, file string, line int) {
	_, _, file, line = pkgFuncFileLine(calldepth+1, false, false)
	return
}

func pkgFuncFileLine(calldepth uint8, inclPkg, inclFunc bool) (subsystem, func0, file string, line int) {
	var pc [1]uintptr
	if runtime.Callers(int(calldepth)+1, pc[:]) < 1 {
		return
	}
	frame, _ := runtime.CallersFrames(pc[:]).Next()
	if frame.PC == 0 {
		return
	}
	file, line = frame.File, frame.Line

	if !(inclPkg || inclFunc) {
		return
	}

	var fpath string
	if file != "" {
		if j := strings.LastIndex(file, "/"); j != -1 {
			fpath = file[0:j]
			file = file[j+1:]
		}
	}

	fnpc := frame.Func
	//if you can find it in the func pointer, then it contains all info:
	//e.g. ugorji.net/ndb/server.(*Ldb).SvrQuery
	if fnpc != nil {
		func0 = fnpc.Name()
		var dot int = -1
		for i := len(func0) - 1; i > 0; i-- {
			if func0[i] == '.' {
				dot = i
			} else if func0[i] == '/' {
				break
			}
		}
		if dot != -1 {
			//override fpath got from file. Sometimes, file information is wrong.
			fpath = func0[:dot]
			func0 = func0[dot+1:]
		}
	}

	if !inclPkg {
		return
	}

	// slash1, slash2, dot, dotAfterLastSlash := -1, -1, -1, -1
	// for i := len(fpath) - 1; i > 0; i-- {
	// 	if fpath[i] == '.' {
	// 		dot = i
	// 	} else if fpath[i] == '/' {
	// 		if slash1 == -1 {
	// 			slash1 = i
	// 			dotAfterLastSlash = dot
	// 			continue
	// 		}
	// 		if slash2 == -1 {
	// 			dotAfterLastSlash = dot
	// 			slash2 = i
	// 			break
	// 		}
	// 	}
	// }
	// //fmt.Printf(">>>> len: %v, slash1: %v, slash2: %v, slash3: %v", len(fpath), slash1, slash2, slash3)
	// if slash2 != -1 {
	// 	if dotAfterLastSlash != -1 && dotAfterLastSlash < slash1 {
	// 		subsystem = fpath[slash1+1:]
	// 	} else {
	// 		subsystem = fpath[slash2+1:]
	// 	}
	// } else if slash1 != -1 {
	// 	subsystem = fpath[slash1+1:]
	// } else {
	// 	subsystem = fpath
	// }

	// a package name can only have: letter digit _
	// so range forward: note last non-package character, and the / after it
	// the package path is from right after that /
	var slashpos, nonpos = -1, -1
	for j, r := range fpath {
		if r == '/' {
			if slashpos == -1 {
				slashpos = j
			}
		} else if !(r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
			nonpos = j
			slashpos = -1
		}
	}
	_ = nonpos
	subsystem = fpath[slashpos+1:]
	return
}

func Stack(bs []byte, all bool) []byte {
	if bs != nil {
		bs = bs[:0]
	}
	newlen := 512
	for {
		if cap(bs) >= newlen {
			bs = bs[:newlen]
		} else {
			bs = make([]byte, newlen)
		}
		i := runtime.Stack(bs, all)
		if i < len(bs) {
			bs = bs[:i+1]
			bs[i] = '\n'
			break
		}
		newlen = len(bs) * 2
	}
	return bs
	// debug.PrintStack()
}

// P printf. the message in red on the terminal.
// Use it in place of fmt.Printf (which it calls internally).
//
// It also adds diagnostics: package, file, line, func:
func P(pattern string, args ...interface{}) {
	var delim string
	if len(pattern) > 0 && pattern[len(pattern)-1] != '\n' {
		delim = "\n"
	}
	p, fn, f, l := PkgFuncFileLine(2)
	fmt.Fprintf(os.Stderr, "\033[1;31m"+fmt.Sprintf(">>gid: %d, %s:%d %s.%s ", curGoroutineID(), f, l, p, fn)+pattern+delim+"\033[0m", args...)
	os.Stderr.Sync()
}

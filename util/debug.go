package util

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

const (
	debugging = true
)

func ValueToErr(panicVal interface{}, err *error) {
	switch xerr := panicVal.(type) {
	case nil:
	case error:
		*err = xerr
	case string:
		*err = errors.New(xerr)
	default:
		*err = fmt.Errorf("%v", panicVal)
	}
	return
}

func Debugf(format string, args ...interface{}) {
	if debugging {
		if len(format) == 0 || format[len(format)-1] != '\n' {
			format = format + "\n"
		}
		fmt.Printf(format, args...)
	}
}

// DebugLineInfo will return the line information.
// Examples:
//   1. func NewDriver() IN ugorji.net/ndb/driver.go
//      subsystem: ndb
//      file/line: driver.go
//      func0:     NewDriver
//   2. func (*Ldb) SvrPut IN ugorji.net/ndb/server/leveldb.go
//      subsystem: ndb/server
//      file/line: leveldb.go
//      func0:     (*Ldb).SvrPut
func DebugLineInfo(calldepth uint8, unsetVal string) (subsystem, file string, line int, func0 string) {
	subsystem, file, func0 = unsetVal, unsetVal, unsetVal
	if calldepth <= 0 {
		return
	}
	pc, file, line, ok := runtime.Caller(int(calldepth))
	if !ok {
		return
	}
	var fpath string
	if file != "" {
		if j := strings.LastIndex(file, "/"); j != -1 {
			fpath = file[0:j]
			file = file[j+1:]
		}
	}
	//if you can find it in the func pointer, then it contains all info:
	//e.g. ugorji.net/ndb/server.(*Ldb).SvrQuery
	fnpc := runtime.FuncForPC(pc)
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

	slash1, slash2, dot, dotAfterLastSlash := -1, -1, -1, -1
	for i := len(fpath) - 1; i > 0; i-- {
		if fpath[i] == '.' {
			dot = i
		} else if fpath[i] == '/' {
			if slash1 == -1 {
				slash1 = i
				dotAfterLastSlash = dot
				continue
			}
			if slash2 == -1 {
				dotAfterLastSlash = dot
				slash2 = i
				break
			}
		}
	}
	//fmt.Printf(">>>> len: %v, slash1: %v, slash2: %v, slash3: %v", len(fpath), slash1, slash2, slash3)
	if slash2 != -1 {
		if dotAfterLastSlash != -1 && dotAfterLastSlash < slash1 {
			subsystem = fpath[slash1+1:]
		} else {
			subsystem = fpath[slash2+1:]
		}
	} else if slash1 != -1 {
		subsystem = fpath[slash1+1:]
	} else {
		subsystem = fpath
	}
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

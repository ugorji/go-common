package runtimeutil

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"sync"
)

// culled from $GOROOT/src/net/http/h2_bundle.go

type goroutineLock uint64

func newGoroutineLock() goroutineLock {
	if !Debug() {
		return 0
	}
	return goroutineLock(GoroutineID())
}

func (g goroutineLock) check() {
	if !Debug() {
		return
	}
	if GoroutineID() != uint64(g) {
		panic("running on the wrong goroutine")
	}
}

func (g goroutineLock) checkNotOn() {
	if !Debug() {
		return
	}
	if GoroutineID() == uint64(g) {
		panic("running on the wrong goroutine")
	}
}

var goroutineSpace = []byte("goroutine ")

func GoroutineID() uint64 {
	bp := littleBuf.Get().(*[]byte)
	defer littleBuf.Put(bp)
	b := *bp
	b = b[:runtime.Stack(b, false)]
	// Parse the 4707 out of "goroutine 4707 ["
	b = bytes.TrimPrefix(b, goroutineSpace)
	i := bytes.IndexByte(b, ' ')
	if i < 0 {
		panic(fmt.Sprintf("No space found in %q", b))
	}
	b = b[:i]
	n, err := strconv.ParseUint(string(b), 10, 64)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse goroutine ID out of %q: %v", b, err))
	}
	return n
}

var littleBuf = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 64)
		return &buf
	},
}

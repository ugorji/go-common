package logging

import (
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ugorji/go-common/ioutil"

	// "runtime/debug"
	"bytes"
	"fmt"
	"strings"

	"github.com/ugorji/go-common/errorutil"
)

// baseHandlerWriter can handle writing to a stream or a file.
type baseHandlerWriter struct {
	fname         string // file name ("" if not a regular opened file)
	w             io.Writer
	w0            io.Writer
	f             *os.File
	bw            *ioutil.BufWriter
	buf           []byte
	flushInterval time.Duration
	tick          *time.Ticker // used
	mu            sync.RWMutex
	closed        uint32 // 0=closed. 1=open. Use mutex/atomic to update.
}

// NewHandlerWriter returns an un-opened writer.
// It returns nil if both w and fname are empty.
// When passed to AddLogger, AddLogger will call its Open method.
func NewHandlerWriter(w io.Writer, fname string, buf []byte, flushInterval time.Duration,
) (hr Handler) {
	if w == nil && fname == "" {
		return nil
	}
	h := baseHandlerWriter{
		w0:            w,
		fname:         fname,
		buf:           buf,
		flushInterval: flushInterval,
	}
	hr = &h
	return
}

func (h *baseHandlerWriter) Open() (err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.w = nil
	if h.w0 != nil {
		h.w = h.w0
	} else if h.fname != "" {
		h.f, err = os.OpenFile(h.fname, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return
		}
		if h.f != nil {
			h.w = h.f
		}
	}
	if h.w == nil {
		return errorutil.String("No Writer for Logging Handler")
	}
	if h.buf != nil {
		h.bw = ioutil.NewBufWriter(h.w, h.buf)
		h.w = h.bw
	}

	// if h.fname == "" {
	// 	if h.buf != nil {
	// 		h.bw = ioutil.NewBufWriter(h.w, h.buf)
	// 	}
	// } else {
	// 	if err = h.openFile(); err != nil {
	// 		return
	// 	}
	// }

	if h.flushInterval > 0 && h.buf != nil && h.tick == nil {
		h.tick = time.NewTicker(h.flushInterval)
		go func() {
			for _ = range h.tick.C {
				h.flush(true)
			}
		}()
	}
	return
}

// func (h *baseHandlerWriter) openFile() (err error) {
// 	//os.Create = OpenFile(name, O_RDWR|O_CREATE|O_TRUNC, 0666)
// 	if h.f, err = os.OpenFile(h.fname, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666); err != nil {
// 		return
// 	}
// 	if h.f == nil {
// 		h.w = nil
// 	} else {
// 		h.w = h.f
// 	}
// 	if h.bw != nil {
// 		h.bw.W = h.w
// 	}
// 	h.closed = false
// 	return
// }

func (h *baseHandlerWriter) Close() (err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.tick != nil {
		h.tick.Stop()
		h.tick = nil
	}
	if h.closed != 0 {
		return
	}
	err = h.flush(false)
	if h.fname == "" {
		return
	}
	if h.f != nil {
		err = errorutil.Multi([]error{err, h.f.Close()}).NonNilError()
	}
	h.w = nil
	// if v, ok := h.f.(io.Closer); ok {
	// 	err = errorutil.Multi([]error{err, v.Close()}).NonNil()
	// }
	h.closed = 1
	return
}

// func (h *baseHandlerWriter) closeIt() (err error) {
// 	if h.closed {
// 		return
// 	}
// 	err = h.flush(false)
// 	if h.fname == "" {
// 		return
// 	}
// 	if h.f != nil {
// 		err = errorutil.Multi([]error{err, h.f.Close()}).NonNil()
// 	}
// 	h.w = nil
// 	// if v, ok := h.f.(io.Closer); ok {
// 	// 	err = errorutil.Multi([]error{err, v.Close()}).NonNil()
// 	// }
// 	return
// }

func (h *baseHandlerWriter) flush(lock bool) (err error) {
	if lock {
		h.mu.Lock()
		defer h.mu.Unlock()
	}
	if h.bw == nil {
		return
	}
	err = h.bw.Flush()
	return
}

// Handle writes record to output.
// If the ctx is a HasHostRequestId or HasId, it writes information about the context.
func (h *baseHandlerWriter) Handle(ctx interface{}, r Record) (err error) {
	// Handle is on the fast path, so use fine-grained locking, and atomic functions if possible
	defer errorutil.OnErrorf(1, &err, nil)
	if atomic.LoadUint32(&h.closed) == 1 {
		return ClosedErr
	}
	//const timeFmt = "2006-01-02 15:04:05.000000"
	const timeFmt = "0102 15:04:05.000000"
	// even if file is deleted or moved, write will not fail on an open file descriptor.
	// so no need to try multiple times.
	var sId string
	switch x := ctx.(type) {
	case HasHostRequestId:
		sId = x.RequestId() // x.HostId() + " " + x.RequestId()
	case HasId:
		sId = x.Id()
	default:
		sId = "-"
	}

	// Take each Message, and ensure that multi-line messages are indented for clarity
	msg := r.Message
	if strings.Index(r.Message, "\n") != -1 {
		var buf bytes.Buffer
		s := r.Message
		// don't use range. it tries to do utf-8 work.
		var i, j int = 0, 0
		for i = 0; i < len(s); i++ {
			// Always add \t in front of each multiline.
			// Someone reading log files knows to remove first \t in each subsequent line.
			if s[i] == '\n' && i+1 < len(s) {
				buf.WriteString(s[j : i+1])
				buf.WriteByte('\t')
				j = i + 1
			}
		}
		buf.WriteString(s[j:])
		msg = string(buf.Bytes())
	}
	var w io.Writer
	// h.w, h.bw must be accessed within a lock
	h.mu.Lock()
	if h.bw == nil {
		w = h.w
	} else {
		w = h.bw
	}
	_, err = fmt.Fprintf(w, "%s %d %s %v %v %v %v:%v] %s\n",
		r.Level.ShortString(), r.Seq, sId, time.Unix(0, r.TimeUnixNano).UTC().Format(timeFmt),
		r.Target, r.ProgramFunc, r.ProgramFile, r.ProgramLine,
		msg)
	if h.flushInterval == 0 {
		if err2 := h.flush(false); err2 != nil {
			if err == nil {
				err = err2
			} else {
				err = errorutil.Multi([]error{err, err2})
			}
		}
	}
	h.mu.Unlock()
	// debug.PrintStack()
	return
}

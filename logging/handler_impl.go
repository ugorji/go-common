package logging

import (
	"context"
	"io"
	"os"
	"strconv"
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

// type baseHandlerFormat uint8

// const (
// 	humanFormat baseHandlerFormat = 2 + iota
// 	csvFormat
// 	jsonFormat
// )

type humanFormatter struct{}

func (h humanFormatter) Format(ctx context.Context, r Record, seqId string) string {
	//const timeFmt = "2006-01-02 15:04:05.000000"
	const timeFmt = "0102 15:04:05.000000"
	// even if file is deleted or moved, write will not fail on an open file descriptor.
	// so no need to try multiple times.
	var sId string = "-"
	if ctx != nil {
		if appctx, ok := ctx.Value(AppContextKey).(hasId); ok {
			sId = appctx.Id()
		}
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
	if len(r.ProgramFile) < 2 {
		return fmt.Sprintf("%c %s %s %v] %s",
			r.Level.ShortString(), seqId, sId, time.Unix(0, r.TimeUnixNano).UTC().Format(timeFmt),
			msg)
	}
	return fmt.Sprintf("%c %s %s %v %v %v %v:%v] %s",
		r.Level.ShortString(), seqId, sId, time.Unix(0, r.TimeUnixNano).UTC().Format(timeFmt),
		r.Target, r.ProgramFunc, r.ProgramFile, r.ProgramLine,
		msg)
}

// baseHandlerWriter can handle writing to a stream or a file.
type baseHandlerWriter struct {
	fname  string // file name ("" if not a regular opened file)
	w      io.Writer
	w0     io.Writer
	f      *os.File
	bw     *ioutil.BufWriter
	ff     Filter
	buf    []byte
	mu     sync.RWMutex
	fmt    Format
	fmter  Formatter
	seq    uint32
	closed uint32 // 1=closed. 0=open. Use mutex/atomic to update.
}

// NewHandlerWriter returns an un-opened writer.
// It returns nil if both w and fname are empty.
// When passed to AddLogger, AddLogger will call its Open method.
//
// if w=nil and fname is <stderr> or <stdout> respectively,
// then write to the standard err or standart out streams respectively.
func NewHandlerWriter(w io.Writer, fname string, fmt Format, ff Filter) *baseHandlerWriter {
	if w == nil {
		switch fname {
		case "<stderr>":
			w = os.Stderr
		case "<stdout>":
			w = os.Stdout
		}
	}
	if w == nil && fname == "" {
		return nil
	}
	// TODO: support more than just HUMAN format
	var fmter Formatter = humanFormatter{}

	// runtimeutil.P("returning new baseHandlerWriter: w: %v, fname: %s", w, fname)
	// debug.PrintStack()
	return &baseHandlerWriter{
		w0:     w,
		fname:  fname,
		fmt:    fmt,
		fmter:  fmter,
		ff:     ff,
		closed: 1,
	}
}

func (h *baseHandlerWriter) Open(buffer uint16) (err error) {
	// defer func() { runtimeutil.P("baseHandlerWriter.Open closed: %d, error: %v", h.closed, err) }()
	// debug.PrintStack()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed == 0 {
		return
	}
	// runtimeutil.P("opening ...")
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
		return NoWriterForHandlerErr
	}
	h.buf = make([]byte, int(buffer))
	if h.buf != nil {
		h.bw = ioutil.NewBufWriter(h.w, h.buf)
		h.w = h.bw
	}
	h.closed = 0

	// if h.fname == "" {
	// 	if h.buf != nil {
	// 		h.bw = ioutil.NewBufWriter(h.w, h.buf)
	// 	}
	// } else {
	// 	if err = h.openFile(); err != nil {
	// 		return
	// 	}
	// }

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
	if h.closed == 1 {
		return
	}
	// runtimeutil.P("closing ...")
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

func (h *baseHandlerWriter) Filter() Filter {
	return h.ff
}

func (h *baseHandlerWriter) Flush() error {
	return h.flush(true)
}

// Handle writes record to output.
// If the ctx is a HasHostRequestId or HasId, it writes information about the context.
func (h *baseHandlerWriter) Handle(ctx context.Context, r Record) (err error) {
	// Handle is on the fast path, so use fine-grained locking, and atomic functions if possible
	defer errorutil.OnError(&err)
	if atomic.LoadUint32(&h.closed) == 1 {
		return ClosedErr
	}
	var w io.Writer
	// h.w, h.bw must be accessed within a lock
	// runtimeutil.P("w: %p, h.w: %p, h.bw: %p, h.w0: %p, h.closed: %d, fname: %s", w, h.w, h.bw, h.w0, h.closed, h.fname)
	recstr := h.fmter.Format(ctx, r, strconv.Itoa(int(atomic.AddUint32(&h.seq, 1))))
	b := make([]byte, len(recstr)+1)
	copy(b, recstr)
	b[len(b)-1] = '\n'
	h.mu.Lock()
	if h.bw == nil {
		w = h.w
	} else {
		w = h.bw
	}
	_, err = w.Write(b)
	// if h.flushInterval == 0 {
	// 	if err2 := h.flush(false); err2 != nil {
	// 		if err == nil {
	// 			err = err2
	// 		} else {
	// 			err = errorutil.Multi([]error{err, err2})
	// 		}
	// 	}
	// }
	h.mu.Unlock()
	// debug.PrintStack()
	return
}

// func NewStderrHandler(f Filter, flush time.Duration, buf []byte, properties map[string]interface{}) (Handler, error) {
// 	hh := NewHandlerWriter(os.Stderr, n, Human, make([]byte, int(y.buffer)), y.flush)
// }

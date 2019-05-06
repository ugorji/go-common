package logging

import (
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/ugorji/go/codec"
	"github.com/ugorji/go-common/ioutil"
	"github.com/ugorji/go-common/osutil"
	"github.com/ugorji/go-common/runtimeutil"

	// "runtime/debug"

	"fmt"
	"strings"

	"github.com/ugorji/go-common/errorutil"
)

//const timeFmt = "2006-01-02 15:04:05.000000"
const timeFmt = "20060102 15:04:05.000000"

var terminalCtxKey = new(int)

// fmtRecordMessage ensures that multi-line messages are indented for clarity
func fmtRecordMessage(s string) string {
	var buf strings.Builder
	var j = strings.IndexByte(s, '\n') + 1
	for j != 0 && j < len(s) {
		buf.WriteString(s[:j])
		buf.WriteByte('\t')
		s = s[j:]
		j = strings.IndexByte(s, '\n') + 1
	}
	if buf.Len() == 0 {
		return s
	}
	buf.WriteString(s)
	return buf.String()
}

func fmtCtxId(ctx context.Context) (sId string) {
	if ctx != nil {
		if appctx, ok := ctx.Value(AppContextKey).(hasId); ok {
			sId = appctx.Id()
		} else if tid := ctx.Value(CorrelationIDContextKey); tid != nil {
			sId = fmt.Sprintf("%v", tid)
		}
	}
	if sId == "" {
		sId = "-"
	}
	return
}

func fmtProgFunc(s string) string {
	// Given any of these 3, return reload
	//    (*watcher).reload.func1
	//    (*watcher).reload
	//    reload
	if i := strings.IndexByte(s, '.'); i != -1 {
		s = s[i+1:]
		if i = strings.IndexByte(s, '.'); i != -1 {
			s = s[:i]
		}
	}
	return s
}

var jsonHandle codec.JsonHandle

type jsonFormatter struct{}

func (h jsonFormatter) Format(ctx context.Context, r *Record, seqId string) []byte {
	// const timeFmt = "20060102 15:04:05.000000"
	var t = struct {
		Seq       string `codec:"q"`
		ContextID string `codec:"id"`
		*Record
		Message string `codec:"m"`
	}{seqId, fmtCtxId(ctx), r, fmtRecordMessage(r.Message)}
	var b []byte
	codec.NewEncoderBytes(&b, &jsonHandle).MustEncode(&t)
	return b
}

type csvFormatter struct{}

func (h csvFormatter) Format(ctx context.Context, r *Record, seqId string) (v []byte) {
	// Seq Level Timestamp Target Func File Line Message
	var s [9]string
	s[0] = seqId
	s[1] = fmtCtxId(ctx)
	s[2] = level2s[r.Level]
	s[3] = r.Time.Format(timeFmt)
	s[4] = r.Target
	s[5] = r.ProgramFunc
	s[6] = r.ProgramFile
	s[7] = strconv.Itoa(int(r.ProgramLine))
	s[8] = fmtRecordMessage(r.Message)

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Write(s[:])
	w.Flush()
	v = buf.Bytes()
	// go's csv writer adds a new line to end of output - strip it
	if v[len(v)-1] == '\n' {
		v = v[:len(v)-1]
	}
	return
}

type humanFormatter struct{}

func (h humanFormatter) Format(ctx context.Context, r *Record, seqId string) []byte {
	// even if file is deleted or moved, write will not fail on an open file descriptor.
	// so no need to try multiple times.
	var sId = fmtCtxId(ctx)
	var color = terminalCtxKey == ctx.Value(terminalCtxKey)
	var s string
	if len(r.ProgramFile) < 2 {
		var fmtstr = "%c %s %s %s %s] %s"
		if color {
			fmtstr = "%c %s %s \033[0;94m%s\033[0m \033[0;93m%s]\033[0m %s"
		}
		s = fmt.Sprintf(fmtstr,
			r.Level.ShortString(), seqId, sId, r.Time.Format(timeFmt),
			r.Target,
			fmtRecordMessage(r.Message))
	} else {
		var fmtstr = "%c %s %s %s %s %s %s:%d] %s"
		if color {
			fmtstr = "%c %s %s \033[0;94m%s\033[0m \033[0;93m%s\033[0m \033[0;92m%s %s:%d]\033[0m %s"
		}
		s = fmt.Sprintf(fmtstr,
			r.Level.ShortString(), seqId, sId, r.Time.Format(timeFmt),
			r.Target, fmtProgFunc(r.ProgramFunc), r.ProgramFile, r.ProgramLine,
			fmtRecordMessage(r.Message))
	}
	return runtimeutil.BytesView(s)
}

// baseHandlerWriter can handle writing to a stream or a file.
type baseHandlerWriter struct {
	fname  string // file name ("" if not a regular opened file)
	w0     io.Writer
	f      *os.File
	bw     *ioutil.BufWriter
	ff     Filter
	buf    []byte
	mu     sync.RWMutex
	fmt    Format
	fmter  Formatter
	seq    uint64
	closed uint32 // 1=closed. 0=open. Use mutex/atomic to update.
	fd     int
	nl     [1]byte
}

// NewHandlerWriter returns an un-opened writer.
// It returns nil if both w and fname are empty.
// When passed to AddLogger, AddLogger will call its Open method.
//
// if w=nil and fname is <stderr> or <stdout> respectively,
// then write to the standard err or standart out streams respectively.
func NewHandlerWriter(w io.Writer, fname string, fmt Format, ff Filter) (h *baseHandlerWriter) {
	var fd int = -1
	if w == nil {
		switch fname {
		case stderr:
			w = os.Stderr
		case stdout:
			w = os.Stdout
		}
	}
	if w != nil {
		fname = ""
		if w == os.Stderr {
			fd = int(os.Stderr.Fd())
		} else if w == os.Stdout {
			fd = int(os.Stdout.Fd())
		}
	} else if fname == "" {
		return nil
	}

	// runtimeutil.P("returning new baseHandlerWriter: w: %v, fname: %s", w, fname)

	h = &baseHandlerWriter{
		w0:     w,
		fname:  fname,
		ff:     ff,
		closed: 1,
		fd:     fd,
	}
	h.nl[0] = '\n'
	h.fmt = fmt
	switch fmt {
	case Human:
		h.fmter = humanFormatter{}
	case JSON:
		h.fmter = jsonFormatter{}
	case CSV:
		h.fmter = csvFormatter{}
	default:
		h.fmt = Human
		h.fmter = humanFormatter{}
	}
	return
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
	h.buf = make([]byte, int(buffer))
	if h.w0 != nil {
		h.bw = ioutil.NewBufWriter(h.w0, h.buf)
	} else if h.fname != "" {
		h.f, err = os.OpenFile(h.fname, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return
		}
		h.bw = ioutil.NewBufWriter(h.f, h.buf)
	} else {
		return NoWriterForHandlerErr
	}
	// h.w = h.bw
	h.closed = 0
	return
}

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
	// if v, ok := h.f.(io.Closer); ok {
	// 	err = errorutil.Multi([]error{err, v.Close()}).NonNil()
	// }
	h.closed = 1
	return
}

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
func (h *baseHandlerWriter) Handle(ctx context.Context, r *Record) (err error) {
	// Handle is on the fast path, so use fine-grained locking, and atomic functions if possible
	defer errorutil.OnError(&err)
	if atomic.LoadUint32(&h.closed) == 1 {
		return ClosedErr
	}
	if osutil.IsTerminal(h.fd) {
		ctx = context.WithValue(ctx, terminalCtxKey, terminalCtxKey)
	}
	rec := h.fmter.Format(ctx, r, strconv.Itoa(int(atomic.AddUint64(&h.seq, 1))))
	h.mu.Lock()
	if _, err = h.bw.Write(rec); err == nil {
		_, err = h.bw.Write(h.nl[:])
	}
	h.mu.Unlock()
	return
}

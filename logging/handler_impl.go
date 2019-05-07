package logging

import (
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

	// "runtime/debug"

	"fmt"
	"strings"

	"github.com/ugorji/go-common/errorutil"
)

// closedErr is returned by a Handler.Handle when logging is closed.
var closedErr = errorutil.String("logging: closed")

//const timeFmt = "2006-01-02 15:04:05.000000"
const timeFmt = "20060102 15:04:05.000000"

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

var jsonHandle = codec.JsonHandle{
	TermWhitespace: true,
	HTMLCharsAsIs:  true,
}

type JSONFormatter struct{}

func (h JSONFormatter) Format(ctx context.Context, r *Record, seqId string, w io.Writer) error {
	// const timeFmt = "20060102 15:04:05.000000"
	var t = struct {
		Seq       string `codec:"q"`
		ContextID string `codec:"id"`
		*Record
		Message string `codec:"m"`
	}{seqId, fmtCtxId(ctx), r, fmtRecordMessage(r.Message)}
	return codec.NewEncoder(w, &jsonHandle).Encode(&t)
}

type CSVFormatter struct{}

func (h CSVFormatter) Format(ctx context.Context, r *Record, seqId string, w io.Writer) (err error) {
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

	ww := csv.NewWriter(w)
	if err = ww.Write(s[:]); err == nil {
		ww.Flush()
		err = ww.Error()
	}
	// v = buf.Bytes()
	// // go's csv writer adds a new line to end of output - strip it
	// if v[len(v)-1] == '\n' {
	// 	v = v[:len(v)-1]
	// }
	return
}

type HumanFormatter struct {
	ANSIColor bool
}

func (h HumanFormatter) Format(ctx context.Context, r *Record, seqId string, w io.Writer) (err error) {
	// even if file is deleted or moved, write will not fail on an open file descriptor.
	// so no need to try multiple times.
	var sId = fmtCtxId(ctx)
	var fmtstr string
	if len(r.ProgramFile) < 2 {
		if h.ANSIColor {
			fmtstr = "%c %s %s \033[0;94m%s\033[0m \033[0;93m%s]\033[0m %s\n"
		} else {
			fmtstr = "%c %s %s %s %s] %s\n"
		}
		_, err = fmt.Fprintf(w, fmtstr,
			r.Level.ShortString(), seqId, sId, r.Time.Format(timeFmt),
			r.Target,
			fmtRecordMessage(r.Message))
	} else {
		if h.ANSIColor {
			fmtstr = "%c %s %s \033[0;94m%s\033[0m \033[0;93m%s\033[0m \033[0;92m%s %s:%d]\033[0m %s\n"
		} else {
			fmtstr = "%c %s %s %s %s %s %s:%d] %s\n"
		}
		_, err = fmt.Fprintf(w, fmtstr,
			r.Level.ShortString(), seqId, sId, r.Time.Format(timeFmt),
			r.Target, fmtProgFunc(r.ProgramFunc), r.ProgramFile, r.ProgramLine,
			fmtRecordMessage(r.Message))
	}
	return
	// return runtimeutil.BytesView(s)
}

// handlerWriter can handle writing to a stream or a file.
type handlerWriter struct {
	fname  string // file name ("" if not a regular opened file)
	w0     io.Writer
	f      *os.File
	bw     *ioutil.BufWriter
	ff     Filter
	buf    []byte
	mu     sync.RWMutex
	fmter  Formatter
	seq    uint64
	closed uint32 // 1=closed. 0=open. Use mutex/atomic to update.
}

// newHandlerWriter returns an un-opened writer.
// It returns nil if both w and fname are empty.
// When passed to AddLogger, AddLogger will call its Open method.
//
// if w=nil and fname is <stderr> or <stdout> respectively,
// then write to the standard err or standart out streams respectively.
func newHandlerWriter(w io.Writer, fname string, fmter Formatter, ff Filter) (h *handlerWriter) {
	if w != nil {
		fname = ""
	} else if fname == "" {
		return nil
	}

	// runtimeutil.P("returning new handlerWriter: w: %v, fname: %s", w, fname)

	h = &handlerWriter{
		w0:     w,
		fname:  fname,
		ff:     ff,
		closed: 1,
	}

	if fmter == nil {
		h.fmter = HumanFormatter{ANSIColor: false}
	} else {
		h.fmter = fmter
	}
	return
}

// NewHandlerWriter returns an un-opened handler.
func NewHandlerWriter(w io.Writer, fmter Formatter, ff Filter) (h *handlerWriter) {
	return newHandlerWriter(w, "", fmter, ff)
}

// NewHandlerFile returns an un-opened handler.
func NewHandlerFile(fname string, fmter Formatter, ff Filter) (h *handlerWriter) {
	return newHandlerWriter(nil, fname, fmter, ff)
}

func (h *handlerWriter) Open(buffer uint16) (err error) {
	// defer func() { runtimeutil.P("handlerWriter.Open closed: %d, error: %v", h.closed, err) }()
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

func (h *handlerWriter) Close() (err error) {
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

func (h *handlerWriter) flush(lock bool) (err error) {
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

func (h *handlerWriter) Filter() Filter {
	return h.ff
}

func (h *handlerWriter) Flush() error {
	return h.flush(true)
}

// Handle writes record to output.
func (h *handlerWriter) Handle(ctx context.Context, r *Record) (err error) {
	// Handle is on the fast path, so use fine-grained locking, and atomic functions if possible
	defer errorutil.OnError(&err)
	if atomic.LoadUint32(&h.closed) == 1 {
		return closedErr
	}
	h.mu.Lock()
	err = h.fmter.Format(ctx, r, strconv.Itoa(int(atomic.AddUint64(&h.seq, 1))), h.bw)
	// if _, err = h.bw.Write(rec); err == nil {
	// 	_, err = h.bw.Write(h.nl[:])
	// }
	h.mu.Unlock()
	return
}

func lhw(f *os.File) Handler {
	return NewHandlerWriter(f,
		// JSONFormatter{},
		// CSVFormatter{},
		HumanFormatter{ANSIColor: osutil.IsTerminal(int(f.Fd()))},
		nil)
}

// BasicInit is used to simply initialize the logging subsystem.
//
// It creates a Handler for each name, logging using the HumanFormatter.
//
//   - If the name is "" or <stderr>, then it logs to standard error stream
//   - Else If the name is <stdout>, then it logs to standard output stream
//   - Else it logs to a file with the name given
func BasicInit(names []string, c Config) (err error) {
	for _, n := range names {
		switch n {
		case "", stderrName:
			err = AddHandler(n, lhw(os.Stderr))
		case stdoutName:
			err = AddHandler(n, lhw(os.Stdout))
		default:
			err = AddHandler(n, NewHandlerFile(n, HumanFormatter{}, nil))
		}
		if err != nil {
			return
		}
	}
	AddLogger("", c.MinLevel, nil, names)
	return Open(c)
}

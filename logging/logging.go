package logging

/*
 NOTE
 - Do not call OnErrorf for LogXXX functions, as these are usually called without
   regard for the error return value.
*/

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ugorji/go-common/errorutil"
	"github.com/ugorji/go-common/runtimeutil"
)

// pkgArgs allows us put all args together, to prevent bugs.
// other code was using mu, not realizing it was a pkg variable.
var y = struct {
	calldepthDelta uint8
	once           sync.Once
	mu             sync.RWMutex
	//lmu sync.Mutex

	flush  time.Duration
	buffer uint16 // size of buffer

	seq uint32

	// defHandler       Handler
	tick *time.Ticker
	// noopLogger *Logger
	loggers  map[string]*logger
	handlers map[string]Handler
	// handlerFactories map[string]HandlerFactory

	// sealed       bool   // once sealed, the system cannot add any more handlers or modify them.
	closed          bool   // once closed, no logging can happen again
	closedUint32    uint32 // closed = 1, open = 0 // mirror of closed, for atomic access
	populatePCLevel Level  // PopulatePCLevel is threshold up to which we log file/line information
	minLevel        Level  // minLevel is default for each implicitly created Logger

	stderrHandlerName string
	stderrHandler     Handler

	stdoutHandlerName string
	stdoutHandler     Handler
}{
	closed:          true, // starts closed
	closedUint32:    1,
	calldepthDelta:  2,
	handlers:        make(map[string]Handler),
	loggers:         make(map[string]*logger),
	flush:           5 * time.Second,
	buffer:          32 << 10, // 32KB, enough for roughly 100 lines
	populatePCLevel: WARNING,
	minLevel:        INFO,
	// handlerFactories: make(map[string]HandlerFactory),
	// noopLogger: &logger{},
}

// ErrorContextKey is the context.Context key used to store an error
var ErrorContextKey = new(int)

// AppContextKey is the context.Context key used to store an app.Context
var AppContextKey = new(int)

var (
	FilterRejectedErr     = errorutil.String("logging: log level lower than logger threshold")
	EmptyMessageErr       = errorutil.String("logging: empty message")
	NoWriterForHandlerErr = errorutil.String("logging: no writer for handler")

	OnlyOneStderrHandlerErr = errorutil.String("logging: only one stderr handler can exist")
	OnlyOneStdoutHandlerErr = errorutil.String("logging: only one stdout handler can exist")

	// ClosedErr is returned if we try to do something when logging is closed.
	// TODO: use this across the board (sometimes we return nil wrongly)
	ClosedErr = errorutil.String("logging: closed")
)

const stderr = "<stderr>"
const stdout = "<stdout>"

type Format uint8

const (
	Human Format = 2 + iota
	CSV
	JSON
)

type hasId interface {
	Id() string
}

type Backtrace struct {
	File string
	Line uint16
}

type Logger struct {
	l *logger
	n string
}

type logger struct {
	name         string
	minLevel     Level
	backtraces   []Backtrace
	handlerNames []string
	handlers     []Handler
}

type Record struct {
	Target      string    `codec:"s"`
	ProgramFile string    `codec:"f"`
	ProgramFunc string    `codec:"c"`
	Message     string    `codec:"m"`
	Time        time.Time `codec:"t"`
	ProgramLine uint16    `codec:"n"`
	Level       Level     `codec:"l"`
	// Seq         uint32 // sequence number has to be a property of the Handle
}

type Formatter interface {
	Format(ctx context.Context, r *Record, seqId string) string
}

// Noop Handler and Filter.
type Noop struct{}

func (n Noop) Handle(ctx context.Context, r *Record) error { return nil }
func (n Noop) Accept(ctx context.Context, r *Record) error { return nil }

// type HandlerFactory func(f Filter, flush time.Duration, buf []byte, properties map[string]interface{}) (Handler, error)

type Handler interface {
	// Name() string
	Handle(ctx context.Context, r *Record) error
	Filter() Filter
	Flush() error
	Close() error
	Open(buffer uint16) error
}

type Filter interface {
	Accept(ctx context.Context, r *Record) error
}

type FilterFunc func(ctx context.Context, r *Record) error

func (f FilterFunc) Accept(ctx context.Context, r *Record) error { return f(ctx, r) }

type HandlerFunc func(ctx context.Context, r *Record) error

func (f HandlerFunc) Handle(ctx context.Context, r *Record) error { return f(ctx, r) }
func (f HandlerFunc) Filter() Filter                              { return nil }
func (f HandlerFunc) Flush() error                                { return nil }
func (f HandlerFunc) Close() error                                { return nil }
func (f HandlerFunc) Open(buffer uint16) error                    { return nil }

// AddHandler will bind a handler to a given name,
// iff no handler is bound to that name.
//
// Note that a Handler is bound one time only.
func AddHandler(name string, f Handler) (err error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	// if y.closed { // || y.sealed {
	// 	return
	// }
	return addHandler(name, f)
}

// addHandler is called within AddHandler or baseLogger, within a lock
//
// Note that only one handler can be attached to os.Stderr or os.Stdout
func addHandler(name string, f Handler) (err error) {
	if _, ok := y.handlers[name]; ok {
		// delete(y.handlers, name)
		return
	}
	if w, ok := f.(*baseHandlerWriter); ok {
		runtimeutil.P("baseHandlerWriter: name: '%s', stdout: %v, stderr: %v", name, w.w0 == os.Stdout, w.w0 == os.Stderr)
		if w.w0 == os.Stderr {
			if y.stderrHandler != nil {
				return OnlyOneStderrHandlerErr
			}
			y.stderrHandlerName = name
			y.stderrHandler = f
		} else if w.w0 == os.Stdout {
			if y.stdoutHandler != nil {
				return OnlyOneStdoutHandlerErr
			}
			y.stdoutHandlerName = name
			y.stdoutHandler = f
		}
	}
	y.handlers[name] = f
	if !y.closed {
		err = f.Open(y.buffer)
	}
	runtimeutil.P("handler: name: '%s'", name)
	return
	// y.handlerFactories[name] = f
}

// AddLogger will return existing Logger by given name, or
// create one if not existing using parameters passed.
//
// If the name "" is not bound to any logger, it is
// created and will serve as the prototype for minLevel and handlers
// if invalid or nil parameters passed.
func AddLogger(name string, minLevel Level, backtraces []Backtrace, handlerNames []string) (l *logger) {
	y.mu.RLock()
	// if y.closed {
	// 	// l = y.noopLogger
	// } else {
	// 	l = y.loggers[name]
	// }
	l = y.loggers[name]
	y.mu.RUnlock()
	if l != nil {
		return l
	}
	y.mu.Lock()
	defer y.mu.Unlock()
	l = &logger{name: name}
	l.backtraces = backtraces
	b := baseLogger()
	if minLevel == INVALID {
		minLevel = b.minLevel
	}
	// minLevel = 0 // test that all debug messages go through
	l.minLevel = minLevel
	if handlerNames == nil {
		l.handlerNames = b.handlerNames
		l.handlers = b.handlers
	} else {
		l.handlerNames = handlerNames
		l.handlers = make([]Handler, 0, 8)
		for _, n := range handlerNames {
			if hh, ok := y.handlers[n]; ok {
				l.handlers = append(l.handlers, hh)
			}
		}
	}
	y.loggers[name] = l
	runtimeutil.P("logger: name: '%s', level: %c, handlers: %v", name, level2c[l.minLevel], l.handlerNames)
	return
}

// this function is only called by baseLogger, called by AddLogger, within a lock
func addBaseLogger(l *logger, n string, hh Handler) {
	l.handlerNames = []string{n}
	l.handlers = []Handler{hh}
	y.loggers[""] = l
}

// baseLogger will return the Logger bound to "".
//
// If none is bound, it will create a Logger bound to "" using the handler
//    - ... bound to a handler "<stderr>" if it exists
//    - ... bound to the handler writing to Stderr
//    - ... new Handler writing to Stderr in Human format (bind it to "<stderr>")
// Note that we do not bind to a handler that is the only one configured,
// as that is not what the user may want e.g. if stackdriver alone is configure,
// user may want to only write error messages from 3 subsystems.
//
// baseLogger is only called by AddLogger, within a lock
func baseLogger() (l *logger) {
	// this is the logger attached to a blank name.
	// if none found, make a new one
	l = y.loggers[""]
	if l != nil {
		return
	}
	l = &logger{minLevel: y.minLevel}
	var n string
	var hh Handler

	n = stderr
	if hh = y.handlers[n]; hh != nil {
		addBaseLogger(l, n, hh)
		return
	}

	// switch len(y.handlers) {
	// case 0:
	// case 1:
	// 	for n, hh = range y.handlers {
	// 		addBaseLogger(l, n, hh)
	// 		return
	// 	}
	// default:
	// 	n, hh = y.stderrHandlerName, y.stderrHandler
	// 	if hh != nil {
	// 		addBaseLogger(l, n, hh)
	// 		return
	// 	}
	// 	// for n, hh = range y.handlers {
	// 	// 	// look for handler writing to stderr with human formatter
	// 	// 	if w, ok := hh.(*baseHandlerWriter); ok && w.w0 == os.Stderr {
	// 	// 		addBaseLogger(l, n, hh)
	// 	// 		return
	// 	// 	}
	// 	// }
	// }

	n, hh = y.stderrHandlerName, y.stderrHandler
	if hh != nil {
		addBaseLogger(l, n, hh)
		return
	}

	// create new one
	n = stderr
	hh = NewHandlerWriter(os.Stderr, n, Human, nil)
	if err := addHandler(n, hh); err != nil {
		runtimeutil.P("error creating/adding os.Stderr Handler for baseLogger: %v", err)
		panic(err)
	}
	addBaseLogger(l, n, hh)
	return
}

func isClosed() bool {
	return atomic.LoadUint32(&y.closedUint32) == 1
}

func PkgLogger() *Logger {
	subsystem, _, _, _ := runtimeutil.PkgFuncFileLine(2)
	return &Logger{n: subsystem}
}

func NamedLogger(name string) *Logger {
	return &Logger{n: name}
}

func FilterByLevel(level Level) FilterFunc {
	x := func(_ context.Context, r *Record) error {
		if r.Level < level {
			// s := "The log record level: %v, is lower than the logger threshold: %v"
			// return fmt.Errorf(s, r.Level, level)
			return FilterRejectedErr
		}
		return nil
	}
	return x
}

func Open(flush time.Duration, buffer uint16, minLevel, populatePCLevel Level) error {
	y.mu.Lock()
	defer y.mu.Unlock()
	if !y.closed {
		return nil
	}

	if y.flush != 0 {
		y.flush = flush
	}
	if y.buffer != 0 {
		y.buffer = buffer
	}
	if populatePCLevel != 0 {
		y.populatePCLevel = populatePCLevel
	}
	if minLevel != 0 {
		y.minLevel = minLevel
	}

	var merrs []error
	f2 := func(h Handler) error { return h.Open(buffer) }
	if err := runAllHandlers(f2); err != nil {
		merrs = append(merrs, err)
	}

	y.tick = time.NewTicker(y.flush)
	go func() {
		for range y.tick.C {
			Flush()
		}
	}()
	y.closed = false
	y.closedUint32 = 0
	return merr(merrs)
}

// runAllHandlers runs a function on each Handler.
// It does not hold onto the locks - so acquire locks if needed.
func runAllHandlers(f func(h Handler) error) error {
	var merrs []error
	for _, h := range y.handlers {
		if err := f(h); err != nil {
			merrs = append(merrs, err)
		}
	}
	return merr(merrs)
}

func Close() error {
	y.mu.Lock()
	defer y.mu.Unlock()
	if y.closed {
		return nil
	}
	y.closed = true
	y.closedUint32 = 1
	y.tick.Stop()
	f := func(h Handler) error { return h.Close() }
	return runAllHandlers(f)
}

func Flush() error {
	f := func(h Handler) error { return h.Flush() }
	y.mu.Lock()
	defer y.mu.Unlock()
	if y.closed {
		return nil
	}
	return runAllHandlers(f)
}

// Reopen will close the system if opened, an then Open it
// using the last configured values (which may be the defaults).
func Reopen() error {
	var merrs []error
	if err := Close(); err != nil {
		merrs = append(merrs, err)
	}
	if err := Open(y.flush, y.buffer, y.minLevel, y.populatePCLevel); err != nil {
		merrs = append(merrs, err)
	}
	return merr(merrs)
}

func merr(merrs []error) error {
	if len(merrs) > 0 {
		return errorutil.Multi(merrs)
	}
	return nil
}

// func flushLoop() {
// }

func (l *logger) logR(calldepth uint8, level Level, ctx context.Context, message string, params ...interface{},
) (err error) {
	// runtimeutil.P("logR called for level: %s, message: %s", level2s[level], message)
	if l == nil || level < l.minLevel {
		return
	}
	// runtimeutil.P("logR l==nil: %v, %s", level2s[level], message)
	if isClosed() {
		return
	}
	if ctx == nil {
		ctx = context.TODO()
	}

	// defer func() { runtimeutil.P("logR error: %v", err) }()

	defer errorutil.OnError(&err)
	if message == "" {
		err = EmptyMessageErr
		return
	}
	if isClosed() {
		return ClosedErr
	}
	var r Record
	var merrs []error
	// No need for lock/unlock here, since handler/filter must ensure it is parallel-safe
	// y.lmu.Lock()
	// defer y.lmu.Unlock()

	for _, h := range y.handlers {
		if ff := h.Filter(); ff != nil && ff.Accept(ctx, &r) != nil {
			continue
		}
		// initialize record iff a handler will accept it
		if r.Message == "" {
			r.Level = level
			r.Target = l.name
			if level == DEBUG || level >= y.populatePCLevel {
				var xpline int
				var xpsubsystem string
				xpsubsystem, r.ProgramFunc, r.ProgramFile, xpline = runtimeutil.PkgFuncFileLine(calldepth + 1)
				_ = xpsubsystem // r.Target = xpsubsystem
				r.ProgramLine = uint16(xpline)
				// check if backtraces necessary
				for _, bt := range l.backtraces {
					if bt.File == r.ProgramFile && bt.Line == r.ProgramLine {
						if y.stderrHandler != nil {
							y.stderrHandler.Flush()
						}
						os.Stderr.Write(debug.Stack()) // debug.PrintStack()
						break
					}
				}
			}
			// if r.Seq == 0 {
			// 	r.Seq = atomic.AddUint32(&y.seq, 1)
			// }
			r.Time = time.Now().UTC()
			if len(params) == 0 {
				r.Message = message
			} else {
				r.Message = fmt.Sprintf(message, params...)
			}
		}
		if herr := h.Handle(ctx, &r); herr != nil {
			merrs = append(merrs, herr)
		}
		// }()
	}
	return merr(merrs)
}

func (l *Logger) ll() *logger {
	if l.l == nil {
		l.l = AddLogger(l.n, 0, nil, nil)
	}
	return l.l
}

// Log is the all-encompassing function that can be used by
// helper log functions in packages without losing caller positon.
//
// A nil *Logger does nothing - equivalent to a no-op.
//
// Example:
//    func logT(message string, params ...interface{}) {
//      logging.Log(nil, 1, level.TRACE, message, params...)
//    }
func (l *Logger) Log(ctx context.Context, calldepth uint8, level Level, message string, params ...interface{}) error {
	return l.ll().logR(y.calldepthDelta+calldepth, level, ctx, message, params...)
}

// func (l *Logger) Trace(ctx context.Context, message string, params ...interface{}) error {
// 	return l.ll().logR(y.calldepthDelta, ctx, TRACE, message, params...)
// }

func (l *Logger) Debug(ctx context.Context, message string, params ...interface{}) error {
	return l.ll().logR(y.calldepthDelta, DEBUG, ctx, message, params...)
}

func (l *Logger) Info(ctx context.Context, message string, params ...interface{}) error {
	return l.ll().logR(y.calldepthDelta, INFO, ctx, message, params...)
}

func (l *Logger) Notice(ctx context.Context, message string, params ...interface{}) error {
	return l.ll().logR(y.calldepthDelta, NOTICE, ctx, message, params...)
}

func (l *Logger) Warning(ctx context.Context, message string, params ...interface{}) error {
	return l.ll().logR(y.calldepthDelta, WARNING, ctx, message, params...)
}

func (l *Logger) Error(ctx context.Context, message string, params ...interface{}) error {
	return l.ll().logR(y.calldepthDelta, ERROR, ctx, message, params...)
}

func (l *Logger) Severe(ctx context.Context, message string, params ...interface{}) error {
	return l.ll().logR(y.calldepthDelta, SEVERE, ctx, message, params...)
}

// Always logs messages at the OFF level, so that it
// always shows in the log (even if logging is turned off)
func (l *Logger) Always(ctx context.Context, message string, params ...interface{}) error {
	return l.ll().logR(y.calldepthDelta, ALWAYS, ctx, message, params...)
}

// Error2 logs an error along with an associated message and possible Trace (if a errorutil.Tracer).
// It is a no-op if err is nil.
func (l *Logger) Error2(ctx context.Context, err error, message string, params ...interface{}) error {
	if err == nil {
		return nil
	}
	return l.ll().logR(y.calldepthDelta, ERROR, context.WithValue(ctx, ErrorContextKey, err), message, params...)

	// var buf bytes.Buffer
	// fmt.Fprintf(&buf, message, params...)
	// buf.WriteString(" :: ")
	// buf.WriteString(err.Error())
	// // switch x := err.(type) {
	// // case errorutil.Tracer:
	// // 	x.ErrorTrace(&buf, "", "")
	// // default:
	// // 	buf.WriteString(err.Error())
	// // }
	// return l.ll().logR(y.calldepthDelta, ERROR, ctx, string(buf.Bytes()))
}

// func waitForAsync() {
// 	tdur, tmax := 1*time.Millisecond, 100*time.Millisecond
// 	for len(y.asyncChan) != 0 {
// 		if tdur < tmax {
// 			tdur *= 2
// 		}
// 		time.Sleep(tdur)
// 	}
// }

// func closeLoggers() error {
// 	if y.closed {
// 		return nil
// 	}
// 	waitForAsync()
// 	y.closeAsyncChan <- struct{}{}
// 	var merrs []error
// 	for _, v := range y.loggers {
// 		if v2, ok := v.Handler.(io.Closer); ok {
// 			if err2 := v2.Close(); err2 != nil {
// 				merrs = append(merrs, err2)
// 			}
// 		}
// 		// delete(y.loggers, k)
// 	}
// 	y.closed = true
// 	if len(merrs) > 0 {
// 		return errorutil.Multi(merrs)
// 	}
// 	return nil
// }

// func openLoggers() error {
// 	if !y.closed {
// 		return nil
// 	}
// 	// waitForAsync()
// 	var merrs []error
// 	for _, v := range y.loggers {
// 		if v2, ok := v.Handler.(Opener); ok {
// 			if err2 := v2.Open(); err2 != nil {
// 				merrs = append(merrs, err2)
// 			}
// 		}
// 		// delete(y.loggers, k)
// 	}
// 	y.closed = false
// 	go asyncLoop()
// 	if len(merrs) > 0 {
// 		return errorutil.Multi(merrs)
// 	}
// 	return nil
// }

// func Reopen() error {
// 	y.mu.Lock()
// 	defer y.mu.Unlock()
// 	// ensure you close and open errors back
// 	return errorutil.Multi([]error{closeLoggers(), openLoggers()}).NonNilError()
// }

// func Open() error {
// 	y.mu.Lock()
// 	defer y.mu.Unlock()
// 	return openLoggers()
// }

// // AddLoggers will add/replace/delete the handlers defined for files and writers specified.
// // (see doc for AddLogger).
// //
// // The files parameter can be one of:
// //   <stderr> : open up logging to stderr
// //   <stdout> : open up logging to stdout
// //   anything else : open up logging to that file
// func AddLoggers(files []string, writers map[string]io.Writer, minLevel Level,
// 	bufsize int, flushInterval time.Duration, async bool) (err error) {
// 	y.mu.Lock()
// 	defer y.mu.Unlock()
// 	if y.closed {
// 		return
// 	}
// 	var loghdlr Handler
// 	for _, logfile := range files {
// 		if logfile == "" {
// 			continue
// 		}
// 		//println("================== LOGFILE: ", logfile)
// 		switch logfile {
// 		case "<stderr>":
// 			loghdlr = NewHandlerWriter(os.Stderr, "", make([]byte, bufsize), flushInterval)
// 		case "<stdout>":
// 			loghdlr = NewHandlerWriter(os.Stdout, "", make([]byte, bufsize), flushInterval)
// 		default:
// 			loghdlr = NewHandlerWriter(nil, logfile, make([]byte, bufsize), flushInterval)
// 		}
// 		//async logging much more performant. Under load, it just becomes a FIFO, which is okay.
// 		//However, we do buffering now, which should eliminate the perf benefits of async.
// 		if err = addLogger(logfile, FilterByLevel(minLevel), loghdlr, async); err != nil {
// 			return
// 		}
// 	}
// 	for n, w := range writers {
// 		if w == nil {
// 			continue
// 		}
// 		loghdlr = NewHandlerWriter(w, "", make([]byte, bufsize), flushInterval)
// 		if err = addLogger(n, FilterByLevel(minLevel), loghdlr, async); err != nil {
// 			return
// 		}
// 	}
// 	return
// }

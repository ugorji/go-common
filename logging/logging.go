/*
 Precise logging package.

 A LogRecord can be sent by the application. It contains a level,
 message, timestamp, target (which subsystem the message came from) and
 PC information (file, line number).

 On the backend, Loggers (encapsulation of filter and handler/writer)
 are registered to a name.

 When a Record is created, it is dispatched to all registered
 Loggers. Each Logger will then publish the record if its filter accepts
 it.

 The easiest way to build a filter is off a Level. There's a convenience
 Filter for that.

 By default, a single logger is initialized bound by name "". It uses a
 built-in handler which writes a single line to the standard output
 stream.

 You can replace a logger by registering a non-nil handler and filter to
 the same name. You can remove a logger by registering a nil handler or
 filter to the same name.

 When a logger is added, the logging framework owns its lifecycle.
 The framework will call Open or Close as needed, especially during calls to
 AddLogger, or Close/Reopen.

 This package is designed to affect the whole process, thus all functions are
 package-level. At init time, it is a no-op. This way, different packages
 are free to use it as needed. A process needs to explicitly add loggers
 in its main() method to activate it.

 The logging package levels are roughly model'ed after syslog. It adds
 TRACE, and removes NOTICE, ALERT and EMERGENCY.

 NOTE

 Most of the helper methods (.Trace, .Debug, .Info, etc) all take a
 Context as the first parameter. Some environments require that context
 e.g. App Engine.

*/
package logging

/*
 NOTE
 - Do not call OnErrorf for LogXXX functions, as these are usually called without
   regard for the error return value.
*/

/*
 TODO:
   - Consider removing all those hooks to HasId, HasHostRequestId, etc
*/

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ugorji/go-common/util"
	"github.com/ugorji/go-common/zerror"
)

// Level is an int representing the log levels. It typically ranges from
// ALL (100) to OFF (107).
type Level uint8

const (
	ALL Level = 100 + iota
	TRACE
	DEBUG
	INFO
	WARNING
	ERROR
	SEVERE
	OFF
)

// pkgArgs allows us put all args together, to prevent bugs.
// other code was using mu, not realizing it was a pkg variable.
type pkgArgs struct {
	calldepthDelta uint8
	mu             sync.RWMutex
	//lmu sync.Mutex
	seq            uint32
	loggers        map[string]logger
	asyncChan      chan asyncLoggable
	closeAsyncChan chan struct{}
	closed         bool
}

var y = pkgArgs{
	calldepthDelta: 2,
	loggers:        make(map[string]logger),
	asyncChan:      make(chan asyncLoggable, 1<<12), //4096
	closeAsyncChan: make(chan struct{}),
}

var (
	PopulatePCLevel = TRACE
	EmptyMessageErr = zerror.String("logging: Empty Message")
	ClosedErr       = zerror.String("logging: closed")
)

func init() {
	// Server is started opened, with no loggers configured by default.
	// We don't setup default logger on stderr, because user may want to use stderr for something else.
	// asyncloop must be running in opened mode. Co-ordinate with openLoggers() function.
	go asyncLoop()
}

type Opener interface {
	Open() error
}

type HasId interface {
	Id() string
}

type HasHostRequestId interface {
	HostId() string
	RequestId() string
}

type Detachable interface {
	Detach() interface{}
}

// logging package has a list of loggers. For each Log Record, it passes it to
// all the loggers in the list. If a logger accepts it (via Filter), then it's
// Handler is called to handle the record (it persist it).
type logger struct {
	//Name    string
	Filter  Filter
	Handler Handler
	Async   bool
}

type Record struct {
	// record is a compact 10 words.
	// It is good for copying ... no pointers, no GC.

	Target       string
	ProgramFile  string
	ProgramFunc  string
	Message      string
	TimeUnixNano int64 //nano seconds since unix epoch
	Seq          uint32
	ProgramLine  uint16
	Level        Level
}

type asyncLoggable struct {
	ctx interface{}
	r   Record
	h   Handler
}

// Noop Handler and Filter.
type Noop struct{}

func (n Noop) Handle(ctx interface{}, r Record) error                           { return nil }
func (n Noop) Accept(ctx interface{}, target string, level Level) (bool, error) { return false, nil }

type Handler interface {
	Handle(ctx interface{}, r Record) error
}

type Filter interface {
	Accept(ctx interface{}, target string, level Level) (bool, error)
}

type HandlerFunc func(ctx interface{}, r Record) error

type FilterFunc func(ctx interface{}, target string, level Level) (bool, error)

func (f HandlerFunc) Handle(ctx interface{}, r Record) error {
	return f(ctx, r)
}

func (f FilterFunc) Accept(ctx interface{}, target string, level Level) (bool, error) {
	return f(ctx, target, level)
}

func ParseLevel(s string) (l Level) {
	switch s {
	case "ALL":
		l = ALL
	case "TRACE":
		l = TRACE
	case "DEBUG":
		l = DEBUG
	case "INFO":
		l = INFO
	case "WARNING":
		l = WARNING
	case "ERROR":
		l = ERROR
	case "SEVERE":
		l = SEVERE
	case "OFF", "":
		l = OFF
	default:
		i := strings.Index(s, ":")
		if i != -1 {
			s = s[:i]
		}
		if i, err := strconv.ParseUint(s, 10, 32); err == nil {
			l = Level(uint32(i))
		} else {
			l = OFF
		}
	}
	return
}

func (l Level) String() (s string) {
	switch l {
	case ALL:
		s = "ALL"
	case TRACE:
		s = "TRACE"
	case DEBUG:
		s = "DEBUG"
	case INFO:
		s = "INFO"
	case WARNING:
		s = "WARNING"
	case ERROR:
		s = "ERROR"
	case SEVERE:
		s = "SEVERE"
	case OFF:
		s = "OFF"
	default:
		s = strconv.Itoa(int(l)) + ":Log_Level"
	}
	return
}

func (l Level) ShortString() (s string) {
	switch l {
	case ALL:
		s = "A"
	case TRACE:
		s = "T"
	case DEBUG:
		s = "D"
	case INFO:
		s = "I"
	case WARNING:
		s = "W"
	case ERROR:
		s = "E"
	case SEVERE:
		s = "S"
	case OFF:
		s = "O"
	default:
		s = strconv.Itoa(int(l))
	}
	return
}

func asyncLoop() {
	for {
		select {
		case x := <-y.asyncChan:
			x.h.Handle(x.ctx, x.r)
		case <-y.closeAsyncChan:
			return
		}
	}
}

// AddLogger will add/replace/delete a new logger to the set.
// It first removes the logger bound to the name (if exists),
// and then adds a new logger if filter and handler are non-nil.
// When removing a logger, it tries to call h.Close().
// When adding a logger, it tries to call h.Open().
func AddLogger(name string, f Filter, h Handler, async bool) (err error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	// don't allow outside users call AddLogger if logging is closed.
	if y.closed {
		return
	}
	return addLogger(name, f, h, async)
}

func addLogger(name string, f Filter, h Handler, async bool) (err error) {
	if l, ok := y.loggers[name]; ok {
		if lo, ok2 := l.Handler.(io.Closer); ok2 {
			if err = lo.Close(); err != nil {
				return
			}
		}
		delete(y.loggers, name)
	}
	if h == nil || f == nil {
		return
	}
	if h != nil && f != nil {
		if lo, ok := h.(Opener); ok {
			if err = lo.Open(); err != nil {
				return
			}
		}
		y.loggers[name] = logger{Filter: f, Handler: h, Async: async}
	}
	return
}

func FilterByLevel(level Level) FilterFunc {
	x := func(_ interface{}, _ string, rLevel Level) (bool, error) {
		if rLevel < level {
			s := "The log record level: %v, is lower than the logger threshold: %v"
			return false, fmt.Errorf(s, rLevel, level)
		}
		return true, nil
	}
	return x
}

func logR(calldepth uint8, ctx interface{}, level Level, message string, params ...interface{},
) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	if message == "" {
		err = EmptyMessageErr
		return
	}
	y.mu.RLock()
	defer y.mu.RUnlock()
	if y.closed {
		return ClosedErr
	}
	var r Record
	var merrs []error
	// No need for lock/unlock here, since handler/filter must ensure it is parallel-safe
	// y.lmu.Lock()
	// defer y.lmu.Unlock()
	for _, l := range y.loggers {
		ok, ferr := l.Filter.Accept(ctx, r.Target, level)
		if ferr != nil {
			merrs = append(merrs, ferr)
			continue
		} else if !ok {
			continue
		}
		//fill out record in 2 steps:
		//- fill out Level and PC info, then call Accept again.
		//- fill out Seq, time and Message (only if this r will be logged)
		if r.Level == 0 {
			r.Level = level
			if level >= PopulatePCLevel && calldepth >= 0 {
				var xpline int
				r.Target, r.ProgramFile, xpline, r.ProgramFunc = util.DebugLineInfo(calldepth+1, "?")
				r.ProgramLine = uint16(xpline)
				//call Accept again, since we now know the target.
				if ok, ferr = l.Filter.Accept(ctx, r.Target, level); ferr != nil {
					merrs = append(merrs, ferr)
					continue
				} else if !ok {
					continue
				}
			}
		}
		if r.Seq == 0 {
			r.Seq = atomic.AddUint32(&y.seq, 1)
			r.TimeUnixNano = time.Now().UnixNano()
			// Testing. Remove.
			// fmt.Printf("====> Params: len: %d\n", len(params))
			// fmt.Printf("====> %#v\n", params)
			if len(params) == 0 {
				r.Message = message
			} else {
				r.Message = fmt.Sprintf(message, params...)
			}
		}
		if l.Async {
			if de, ok := ctx.(Detachable); ok {
				dctx := de.Detach()
				y.asyncChan <- asyncLoggable{dctx, r, l.Handler}
				continue
			}
		}
		// No need for lock/unlock here, since handler must ensure it is parallel-safe
		// func() {
		// y.lmu.Lock()
		// defer y.lmu.Unlock()
		if herr := l.Handler.Handle(ctx, r); herr != nil {
			merrs = append(merrs, herr)
		}
		// }()
	}
	if len(merrs) > 0 {
		err = zerror.Multi(merrs)
	}
	return
}

// Log is the all-encompassing function that can be used by
// helper log functions in packages without losing caller positon.
//
// Example:
//    func logT(message string, params ...interface{}) {
//      logging.Log(nil, 1, level.TRACE, message, params...)
//    }
func Log(ctx interface{}, calldepth uint8, level Level, message string, params ...interface{}) error {
	return logR(y.calldepthDelta+calldepth, ctx, level, message, params...)
}

func Trace(ctx interface{}, message string, params ...interface{}) error {
	return logR(y.calldepthDelta, ctx, TRACE, message, params...)
}

func Debug(ctx interface{}, message string, params ...interface{}) error {
	return logR(y.calldepthDelta, ctx, DEBUG, message, params...)
}

func Info(ctx interface{}, message string, params ...interface{}) error {
	return logR(y.calldepthDelta, ctx, INFO, message, params...)
}

func Warning(ctx interface{}, message string, params ...interface{}) error {
	return logR(y.calldepthDelta, ctx, WARNING, message, params...)
}

func Error(ctx interface{}, message string, params ...interface{}) error {
	return logR(y.calldepthDelta, ctx, ERROR, message, params...)
}

// Error2 logs an error along with an associated message and possible Trace (if a zerror.Tracer).
// It is a no-op if err is nil.
func Error2(ctx interface{}, err error, message string, params ...interface{}) error {
	if err == nil {
		return nil
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, message, params...)
	buf.WriteString(" :: ")
	buf.WriteString(err.Error())
	// switch x := err.(type) {
	// case zerror.Tracer:
	// 	x.ErrorTrace(&buf, "", "")
	// default:
	// 	buf.WriteString(err.Error())
	// }
	return logR(y.calldepthDelta, ctx, ERROR, string(buf.Bytes()))
}

func Severe(ctx interface{}, message string, params ...interface{}) error {
	return logR(y.calldepthDelta, ctx, SEVERE, message, params...)
}

// Always logs messages at the OFF level, so that it
// always shows in the log (even if logging is turned off)
func Always(ctx interface{}, message string, params ...interface{}) error {
	return logR(y.calldepthDelta, ctx, OFF, message, params...)
}

func waitForAsync() {
	tdur, tmax := 1*time.Millisecond, 100*time.Millisecond
	for len(y.asyncChan) != 0 {
		if tdur < tmax {
			tdur *= 2
		}
		time.Sleep(tdur)
	}
}

func closeLoggers() error {
	if y.closed {
		return nil
	}
	waitForAsync()
	y.closeAsyncChan <- struct{}{}
	var merrs []error
	for _, v := range y.loggers {
		if v2, ok := v.Handler.(io.Closer); ok {
			if err2 := v2.Close(); err2 != nil {
				merrs = append(merrs, err2)
			}
		}
		// delete(y.loggers, k)
	}
	y.closed = true
	if len(merrs) > 0 {
		return zerror.Multi(merrs)
	}
	return nil
}

func openLoggers() error {
	if !y.closed {
		return nil
	}
	// waitForAsync()
	var merrs []error
	for _, v := range y.loggers {
		if v2, ok := v.Handler.(Opener); ok {
			if err2 := v2.Open(); err2 != nil {
				merrs = append(merrs, err2)
			}
		}
		// delete(y.loggers, k)
	}
	y.closed = false
	go asyncLoop()
	if len(merrs) > 0 {
		return zerror.Multi(merrs)
	}
	return nil
}

func Reopen() error {
	y.mu.Lock()
	defer y.mu.Unlock()
	// ensure you close and open errors back
	return zerror.Multi([]error{closeLoggers(), openLoggers()}).NonNilError()
}

func Close() error {
	y.mu.Lock()
	defer y.mu.Unlock()
	return closeLoggers()
}

// func Open() error {
// 	y.mu.Lock()
// 	defer y.mu.Unlock()
// 	return openLoggers()
// }

// AddLoggers will add/replace/delete the handlers defined for files and writers specified.
// (see doc for AddLogger).
//
// The files parameter can be one of:
//   <stderr> : open up logging to stderr
//   <stdout> : open up logging to stdout
//   anything else : open up logging to that file
func AddLoggers(files []string, writers map[string]io.Writer, minLevel Level,
	bufsize int, flushInterval time.Duration, async bool) (err error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	if y.closed {
		return
	}
	var loghdlr Handler
	for _, logfile := range files {
		if logfile == "" {
			continue
		}
		//println("================== LOGFILE: ", logfile)
		switch logfile {
		case "<stderr>":
			loghdlr = NewHandlerWriter(os.Stderr, "", make([]byte, bufsize), flushInterval)
		case "<stdout>":
			loghdlr = NewHandlerWriter(os.Stdout, "", make([]byte, bufsize), flushInterval)
		default:
			loghdlr = NewHandlerWriter(nil, logfile, make([]byte, bufsize), flushInterval)
		}
		//async logging much more performant. Under load, it just becomes a FIFO, which is okay.
		//However, we do buffering now, which should eliminate the perf benefits of async.
		if err = addLogger(logfile, FilterByLevel(minLevel), loghdlr, async); err != nil {
			return
		}
	}
	for n, w := range writers {
		if w == nil {
			continue
		}
		loghdlr = NewHandlerWriter(w, "", make([]byte, bufsize), flushInterval)
		if err = addLogger(n, FilterByLevel(minLevel), loghdlr, async); err != nil {
			return
		}
	}
	return
}

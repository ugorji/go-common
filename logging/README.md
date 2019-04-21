# go-common/logging

This repository contains the `go-common/logging` library (or command).

To install:

```
go get github.com/ugorji/go-common/logging
```

# Package Documentation


Precise logging package.

A LogRecord can be sent by the application. It contains a level, message,
timestamp, target (which subsystem the message came from) and PC information
(file, line number).

On the backend, Loggers (encapsulation of filter and handler/writer) are
registered to a name.

When a Record is created, it is dispatched to all registered Loggers. Each
Logger will then publish the record if its filter accepts it.

The easiest way to build a filter is off a Level. There's a convenience
Filter for that.

By default, a single logger is initialized bound by name "". It uses a
built-in handler which writes a single line to the standard output stream.

You can replace a logger by registering a non-nil handler and filter to the
same name. You can remove a logger by registering a nil handler or filter to
the same name.

When a logger is added, the logging framework owns its lifecycle. The
framework will call Open or Close as needed, especially during calls to
AddLogger, or Close/Reopen.

This package is designed to affect the whole process, thus all functions are
package-level. At init time, it is a no-op. This way, different packages are
free to use it as needed. A process needs to explicitly add loggers in its
main() method to activate it.

The logging package levels are roughly model'ed after syslog. It adds TRACE,
and removes NOTICE, ALERT and EMERGENCY.


## NOTE

Most of the helper methods (.Trace, .Debug, .Info, etc) all take a Context
as the first parameter. Some environments require that context e.g. App
Engine.

## Exported Package API

```go
var PopulatePCLevel = TRACE ...
func AddLogger(name string, f Filter, h Handler, async bool) (err error)
func AddLoggers(files []string, writers map[string]io.Writer, minLevel Level, bufsize int, ...) (err error)
func Always(ctx interface{}, message string, params ...interface{}) error
func Close() error
func Debug(ctx interface{}, message string, params ...interface{}) error
func Error(ctx interface{}, message string, params ...interface{}) error
func Error2(ctx interface{}, err error, message string, params ...interface{}) error
func Info(ctx interface{}, message string, params ...interface{}) error
func Log(ctx interface{}, calldepth uint8, level Level, message string, ...) error
func Reopen() error
func Severe(ctx interface{}, message string, params ...interface{}) error
func Trace(ctx interface{}, message string, params ...interface{}) error
func Warning(ctx interface{}, message string, params ...interface{}) error
type Config struct{ ... }
type Detachable interface{ ... }
type Filter interface{ ... }
type FilterFunc func(ctx interface{}, target string, level Level) (bool, error)
    func FilterByLevel(level Level) FilterFunc
type Handler interface{ ... }
    func NewHandlerWriter(w io.Writer, fname string, buf []byte, flushInterval time.Duration) (hr Handler)
type HandlerFunc func(ctx interface{}, r Record) error
type HasHostRequestId interface{ ... }
type HasId interface{ ... }
type Level uint8
    const ALL Level = 100 + iota ...
    func ParseLevel(s string) (l Level)
type Noop struct{}
type Opener interface{ ... }
type Record struct{ ... }
```

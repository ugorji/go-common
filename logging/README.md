# go-common/logging

This repository contains the `go-common/logging` library (or command).

To install:

```
go get github.com/ugorji/go-common/logging
```

# Package Documentation


Package logging provides a precise logging framework.


## LogRecord

A LogRecord can be sent by the application. It contains a level, message,
timestamp, target (which subsystem the message came from) and PC information
(file, line number).


## Filter

A Filter can determine whether a LogRecord should be accepted or not.


## Handle

A Handle can persist a LogRecord as it deems fit. Each Handle can have a
(set of) Filters configured. The Handle will log a record iff all Filters
accept it. The lifecycle of a Handle is: init, open, log OR flush ...,
close.

Multiple Handles can be configured on a running system.

All Handles will write to an in-memory buffer which can typically hold up to
20 log records. When the buffer cannot add another log record, it is
flushed.

Once a Handle has been created for a given name, it cannot be replaced.

This allows you to have different Handles who can log based on different
criteria e.g. stackdriver only logs error and severe messages from web
container at night.


## Formatter

A Formatter can take a LogRecord and convert it to a string for easy
persisting.

A Handle *may* have a Formatter determine how the LogRecord should be
persisted.


## Logger

A Logger can be retrieved for any subsystem (target). This can be retrieved
explicitly by name or implicitly (where we use the package path to infer the
subsystem).

A Logger can also have a Filter attached to it, which determines whether to
send the LogRecord to the Handlers to persist.

To allow short-circuiting creating a LogRecord, a Logger has a minimum log
level defined, so it can bypass logging quickly.

There is no hierachy of Loggers.

Once a Logger has been retrieved for a given subsystem, it cannot be
replaced.


## Flushing

The framework has a timer that will flush each Handle when triggered.


## Framework Initialization

The logging framework is typically initialized by the running application ie
early in its main method.

To make this easier, the logging framework provides an initialization
function taking Config objects that can be configured via json.

The first time that a LogRecord is to be published, it will ensure that the
logging framework is initialized. If it was not initialized apriori, then it
is initialized to have a single Handle that writes a single human readable
line for each log record with minimum level of INFO.

## A Handler factory is registered by passing in a function that takes

  - Properties map[string]interface{}
  - Filter ... (set of Filters)

When a Logger is configured to use a handler, that handler is created and
initialized lazily at first use.

Note that this affect the whole process.


## Framework Runtime

At startup, logging is closed.

It must be started explicitly by calling logging.Reopen(flushInterval,
buffersize).


## Levels based on syslog

The levels are roughly model'ed after syslog. We however remove ALERT and
EMERGENCY.


## Context

All the logging methods (.Trace, .Debug, .Info, etc) take a Context as the
first parameter.

This allows us grab information from the context where appropriate e.g. App
Engine, HTTP Request, etc.


## Debugging

When messages are logged at debug level and lower, we include the program
counter file/line info.


## Typical Usage

Initialization:

```go
    logging.Addhandler(...)...
    logging.Addhandler("", ...)
    logging.AddLogger("", ...)
    logging.Open(flushInterval, buffersize)
    // ...
    logging.Close()
```

Usage:

```go
    Logger log = logging.PkgLogger()
    log.Info(ctx, formatString, params...)
    log.Info(nil, formatString, params...)
```

## Exported Package API

```go
var FilterRejectedErr = errorutil.String("logging: log level lower than logger threshold") ...
var AppContextKey = new(int)
var ErrorContextKey = new(int)
func AddHandler(name string, f Handler) (err error)
func Close() error
func Flush() error
func Open(flush time.Duration, buffer uint16, populatePC Level) error
func Reopen() error
func NewHandlerWriter(w io.Writer, fname string, fmt Format, ff Filter) *baseHandlerWriter
type Config struct{ ... }
type Filter interface{ ... }
type FilterFunc func(ctx context.Context, r Record) error
    func FilterByLevel(level Level) FilterFunc
type Format uint8
    const Human Format = 2 + iota ...
type Formatter interface{ ... }
type Handler interface{ ... }
type HandlerFunc func(ctx context.Context, r Record) error
type Level uint8
    const INVALID Level = 0 ...
    func ParseLevel(s string) (l Level)
type Logger struct{ ... }
    func AddLogger(name string, minLevel Level, backtraces []backtrace, handlerNames []string) (l *Logger)
    func NamedLogger(name string) *Logger
    func PkgLogger() *Logger
type Noop struct{}
type Record struct{ ... }
```

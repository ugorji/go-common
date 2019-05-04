/*
Package logging provides a precise logging framework.

LogRecord

A LogRecord can be sent by the application. It contains a level,
message, timestamp, target (which subsystem the message came from) and
PC information (file, line number).

Filter

A Filter can determine whether a LogRecord should be accepted or not.

Handler

A Handler can persist a LogRecord as it deems fit.
Each Handler can have a (set of) Filters configured.
The Handler will log a record iff all Filters accept it.
The lifecycle of a Handler is: init, open, log OR flush ..., close.

Multiple Handles can be configured on a running system.

All Handles will write to an in-memory buffer which can typically hold up to
20 log records. When the buffer cannot add another log record, it is flushed.

Once a Handler has been created for a given name, it cannot be replaced.

This allows you to have different Handles who can log based on different criteria
e.g. stackdriver only logs error and severe messages from web container at night.

NOTE: The framework *tries* to ensure that only one handler is writing to standard
error or output streams. This allows us to handle backtraces, as we can ensure
that the Handler writing to the standard error stream is flushed first.

Formatter

A Formatter can take a LogRecord and convert it to a string for easy persisting.

A Handler *may* have a Formatter determine how the LogRecord should be persisted.

Logger

A Logger can be retrieved for any subsystem (target). This can be retrieved explicitly
by name or implicitly (where we use the package path to infer the subsystem).

Note that a Logger is only lazily initialized on first use, not on declaration/assignment.
This ensures that the logging framework is initialized before the first log message is called.

A Logger can also have a Filter attached to it, which determines whether to send
the LogRecord to the Handlers to persist.

To allow short-circuiting creating a LogRecord, a Logger has a minimum log level defined,
so it can bypass logging quickly.

There is no hierachy of Loggers.

Once a Logger has been retrieved for a given subsystem, it cannot be replaced.

Backtraces

A Logger can be configured to write a backtrace to standard error stream
at the point that a Log is being generated on a given line in a given go file,
and at a Level >= the Level for populating the file/line PC info in the Record.

File and Line PC information in logs

Each log Record will contain File and Line PC information if the Level == DEBUG
or if Level >= populatePCLevel.

Flushing

The framework has a timer that will flush each Handler when triggered.

Also, each Handle is expected to use a buffer to cache its output.
When the buffer is full, it is flushed. Also, the framework timer will
flush so that you always see log persisted within the timer schedule duration.

Framework Initialization

The logging framework is typically initialized by the running application
ie early in its main method.

To make this easier, the logging framework provides an initialization
function taking Config objects that can be configured via json.

The first time that a LogRecord is to be published, it will ensure that
the logging framework is initialized. If it was not initialized apriori,
then it is initialized to have a single Handler that writes a single human
readable line for each log record with minimum level of INFO.

A Handler factory is registered by passing in a function that takes
  - Properties map[string]interface{}
  - Filter ... (set of Filters)

When a Logger is configured to use a handler, that handler is created and
initialized lazily at first use.

Note that this affect the whole process. 

Framework Runtime

At startup, logging is closed.

It must be started explicitly by calling logging.Reopen(flushInterval, buffersize).

Levels based on syslog

The levels are roughly model'ed after syslog.
We however remove ALERT and EMERGENCY.

Context

All the logging methods (.Trace, .Debug, .Info, etc) take a
Context as the first parameter. 

This allows us grab information from the context where appropriate
e.g. App Engine, HTTP Request, etc.

Debugging

When messages are logged at debug level and lower, we include the
program counter file/line info.

Typical Usage

Initialization:

  logging.Addhandler(...)...
  logging.Addhandler("", ...)
  logging.AddLogger("", ...)
  logging.Open(flushInterval, buffersize)
  // ...
  logging.Close()

Usage:

  // make log a package-level variable
  var log = logging.PkgLogger()
  // Use these within your function calls
  log.Info(ctx, formatString, params...)
  log.Info(nil, formatString, params...)

*/
package logging

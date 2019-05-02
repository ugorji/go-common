/*
Package logging provides a precise logging framework.

A LogRecord can be sent by the application. It contains a level,
message, timestamp, target (which subsystem the message came from) and
PC information (file, line number).

A Filter can determine whether a LogRecord should be accepted or not.

A Formatter can take a LogRecord and convert it to a string for easy persisting.

A Handle can persist a LogRecord as it deems fit.
Each Handle can have a (set of) Filters configured. The Handle will log
a record iff all Filters accept it.
A Handle *may* have a Formatter determine how the LogRecord should be persisted.
The lifecycle of a Handle is: init, log OR flush ..., close.

Multiple Handles can be configured on a running system.

A Logger can be retrieved for any subsystem (target). This can be retrieved explicitly
by name or implicitly (where we use the package path to infer the subsystem).

A Logger can also have a Filter attached to it, which determines whether to send
the LogRecord to the Handlers to persist.

To allow short-circuiting creating a LogRecord, a Logger has a minimum log level defined,
so it can bypass logging quickly.

There is no hierachy of Loggers.

Typical Usage:

  Logger log = logging.PkgLogger()
  log.Info(ctx, formatString, params...)

The logging framework is typically initialized by the running application
ie early in its main method.

To make this easier, the logging framework provides an initialization
function taking Config objects that can be configured via json.

The first time that a LogRecord is to be published, it will ensure that
the logging framework is initialized. If it was not initialized apriori,
then it is initialized to have a single Handle that writes a single human
readable line for each log record with minimum level of INFO.

All Handles will write to an in-memory buffer which can typically hold up to
20 log records. When the buffer cannot add another log record, it is flushed.
Also, the framework has a timer that will flush each Handle when triggered.




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

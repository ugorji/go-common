/*
Package logging provides a precise logging framework.

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

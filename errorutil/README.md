# go-common/errorutil

This repository contains the `go-common/errorutil` library.

To install:

```
go get github.com/ugorji/go-common/errorutil
```

# Package Documentation


Package errorutil contains functions and types for error management.

In general, idiomatic error handling in go follows the following principles:

  - Panic for exceptional conditions. These include:
    Unexpected/invalid input parameters,
    ...
  - Return errors for expected conditions e.g.
    file may not exist,
    network may be down

To be explicit, do not return errors for invalid inputs. Instead, check the
inputs and panic if invalid. This makes the APIs better (as error checking
does not have to be done). It also ensures that input validation should be
done by the caller.

Errors occur as a result of a number of things:

  - An action generated an error which MUST be reported to callers with more context
  - An action generated an error which MUST be reported to callers AS IS
  - Something bad happened which MUST be reported to callers

When reporting an error with more context, user MUST add the action being
performed

```
    e.g. "opening passwd.txt"
```

Generated errors SHOULD include succinct context information (if possible).
This way, caller SHOULD NOT need to re-compute context of the error.

```
    e.g. "missing read permission", "end of file", "index out of bounds (5/4)", "mismatch: 7.5/7.1"
```

There are times when it is idiomatic to report an error AS IS. These include
times where context is not necessary, as the function does a pass-through to
another function.

In general, a rich error contains the following:

  - what action was being performed?
  - what error (if any) occurred while performing this action?
  - where in the code did this occur?

In general, follow these rules when generating errors:

  - Error messages are always in lower case
  - never have "error" in front of them
  - always fit in a single line (We already know it's an error message)
  - Always are of form: "generting UUID: got time of 0 unexpectedly"
  - When generated, there is no "action" (The user already knows what action they called.
    "action" only comes to play when propagating an error)
  - When propagating an error, always propagate a new error appropriately (do not "blindly" throw errors).
  - Errors should be infrequent, so it is ok to determine whether to include
    "context" information when creating errors. I always do.

## This package just provides some helpers for this principle above

  - Wrapper that exposes its wrapped error when Unwrap is called.
    We did this so it would be aligned with go 1.13 xerrors package.

## Exported Package API

```go
func Base(err error) error
func OnError(err *error)
func OnErrorf(err *error, message string, params ...interface{})
type Context struct{ ... }
type Multi []error
type Rich struct{ ... }
    func NewRich(action string, cause error) *Rich
type String string
type Wrapper interface{ ... }
```

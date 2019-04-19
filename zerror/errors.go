// package zerror contains functions and types for error management.
//
// In general, idiomatic error handling in go follows the following principles:
//   - Panic for exceptional conditions. These include:
//     Unexpected/invalid input parameters
//     ...
//   - Return errors for expected conditions e.g.
//     file may not exist
//     network may be down
//
// To be explicit, do not return errors for invalid inputs. Instead, check the inputs
// and panic if invalid. This makes the APIs better (as error checking does not have to be done).
// It also ensures that input validation should be done by the caller.
//
// Errors occur as a result of a number of things:
//   - An action generated an error which MUST be reported to callers with more context
//   - An action generated an error which MUST be reported to callers AS IS
//   - Something bad happened which MUST be reported to callers
//
// When reporting an error with more context, user MUST add the action being performed
//   e.g. "opening passwd.txt"
//
// Generated errors SHOULD include succinct context information (if possible).
// This way, caller SHOULD NOT need to re-compute context of the error.
//   e.g. "missing read permission", "end of file", "index out of bounds (5/4)", "mismatch: 7.5/7.1"
//
// There are times when it is idiomatic to report an error AS IS. These include times
// where context is not necessary, as the function does a pass-through to another function.
//
// In general, a rich error contains the following:
//   - what action was being performed?
//   - what error (if any) occurred while performing this action?
//   - where in the code did this occur?
//
// In general, follow these rules when generating errors:
//   - Error messages are always in lower case
//   - never have "error" in front of them
//   - always fit in a single line (We already know it's an error message)
//   - Always are of form: "generting UUID: got time of 0 unexpectedly"
//   - When generated, there is no "action" (The user already knows what action they called.
//     "action" only comes to play when propagating an error)
//   - When propagating an error, always propagate a new error appropriately (do not "blindly" throw errors).
//   - Errors should be infrequent, so it is ok to determine whether to include
//     "context" information when creating errors. I always do.
package zerror

import (
	"fmt"
	"strings"
	"time"

	"github.com/ugorji/go-common/util"
)

//Multi is a slice of errors, which acts as a single error
type Multi []error

// String wraps a string as an error
type String string

// Context is time and location in code where an error occurred
type Context struct {
	Subsystem string
	File      string
	FuncName  string
	Line      int
	Time      time.Time
}

// Rich is a rich error encapsulating a cause, program context and an optional cause.
type Rich struct {
	// Self    interface{} // use fmt.Sprintf to get it

	// Action is what was being performed e.g. "opening passwd.txt", "checking time"
	Action string
	// Cause is the encapsulated error e.g. "incorrect permissions", "end of file reached"
	Cause error
	// Context is where in the code the error occurred
	Context *Context
}

func (e *Rich) Error() (s string) {
	return e.msg(true)
}

// func (e *Rich) Message() (s string) {
// 	return e.msg(true)
// }

func (e *Rich) msg(ctx bool) string {
	b := make([]byte, 0, 32)
	addColon := false
	if ctx && e.Context != nil {
		b = append(b, e.Context.String()...)
		addColon = true
	}
	if e.Action != "" {
		if addColon {
			b = append(b, ": "...)
		} else {
			addColon = true
		}
		b = append(b, e.Action...)
	}
	if e.Cause != nil {
		if addColon {
			b = append(b, ": "...)
		}
		b = append(b, e.Cause.Error()...)
	}
	return string(b)
}

// SetContext, only if depth >= 0.
func (e *Rich) setContext(depth int8) {
	if depth < 0 {
		return
	}
	x := Context{}
	x.Subsystem, x.File, x.Line, x.FuncName = util.DebugLineInfo(uint8(depth)+1, "")
	e.Context = &x
}

func newRich(action string, cause error, depth int8) *Rich {
	w := &Rich{Action: action, Cause: cause}
	w.setContext(depth)
	return w
}

// New returns a new Err, with context added if depth >= 0
func NewRich(action string, cause error) *Rich {
	return newRich(action, cause, 2)
}

// Base returns the underlying cause of an error.
// If a *Err, it returns the base of its cause.
// Else it returns the error passed.
func Base(err error) error {
	if err != nil {
		if x, ok := err.(*Rich); ok && x.Cause != nil {
			return Base(x.Cause)
		}
	}
	return err
}

func (em Multi) Error() string {
	return fmt.Sprintf("%v errors: %v", len(em), []error(em))
}

// func (em Multi) String() string {
// 	return fmt.Sprintf("%v", []error(em))
// }

func (e Multi) HasError() (b bool) {
	for i := range e {
		if e[i] != nil {
			b = true
			break
		}
	}
	return
}

func (e Multi) NonNilError() error {
	if merrs := e.NonNil(); len(merrs) > 0 {
		return merrs
	}
	return nil
}

// Returns the subset of this Multi which are non nil.
// Note that this is not same as err=nil if they are all nil.
// Use NonNilError if you need to pass a nil value if non-nils.
func (e Multi) NonNil() Multi {
	var merrs Multi
	for _, err := range e {
		if err == nil {
			continue
		}
		switch x := err.(type) {
		case Multi:
			merrs = append(merrs, x.NonNil()...)
		default:
			merrs = append(merrs, err)
		}
	}
	return merrs
}

func (e Multi) First() error {
	if len(e) == 0 {
		return nil
	}
	return e[0]
}

func (e String) Error() string {
	return string(e)
}

// String returns a string containing fields of the *Context (subsystem, file, line, etc)
// e.g. cart [file.go:123 (*struc).Name]
func (x *Context) String() string {
	if x == nil {
		return ""
	}
	if x.Time.IsZero() {
		return fmt.Sprintf("%v [%v:%v %v] ", x.Subsystem, x.File, x.Line, x.FuncName)
	}
	return fmt.Sprintf("%v %v [%v:%v %v] ", x.Time, x.Subsystem, x.File, x.Line, x.FuncName)
}

// func NonNil(errs ...interface{}) error {
// 	var merrs Multi
// 	for _, x := range errs {
// 		if err, _ := x.(error); err != nil {
// 			merrs = append(merrs, err)
// 		}
// 	}
// 	return merrs
// }

// OnErrorf is called to enhance the error passed.
// If the passed in error is nil, do nothing.
// Else Wrap it with context information.
// Most callers use it from defer functions.
func OnErrorf(calldepth int8, err *error, msgAndParams ...interface{}) {
	if *err == nil {
		return
	}
	var message string
	switch {
	case len(msgAndParams) == 0,
		len(msgAndParams) == 1 && msgAndParams[0] == nil:
	default:
		message, _ = msgAndParams[0].(string)
		switch {
		case message == "":
			message = fmt.Sprint(msgAndParams...)
		case len(msgAndParams) > 1:
			switch {
			case strings.IndexByte(message, '%') == -1:
				message = fmt.Sprint(msgAndParams...)
			default:
				message = fmt.Sprintf(message, msgAndParams[1:]...)
			}
		}
	}
	if calldepth >= 0 {
		calldepth++
	}
	// err1 := New(message, *err, calldepth)
	var err1 error = newRich(message, *err, calldepth+1)
	*err = err1
}

// func OnErrorf(calldepth int8, err *error, msg interface{}, parameters ...interface{}) {
// 	if *err == nil {
// 		return
// 	}
// 	if calldepth >= 0 {
// 		calldepth++
// 	}
// 	message, _ := msg.(string)
// 	if message != "" {
// 		message = fmt.Sprintf(message, parameters...)
// 	}
// 	*err = New(message, *err, calldepth)
// }

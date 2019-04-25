package errorutil

import (
	"fmt"
	"strings"
	"time"

	"github.com/ugorji/go-common/runtimeutil"
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
	x.Subsystem, x.File, x.Line, x.FuncName = runtimeutil.DebugLineInfo(uint8(depth)+1, "")
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

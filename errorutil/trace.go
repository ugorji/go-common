//+build ignore

package errorutil

import (
	"bytes"
	"io"
)

type Tracer interface {
	// Trace writes details of an error (e.g. an Chain) to the writer.
	ErrorTrace(w io.Writer, prefix, indent string) error
}

func (e *Err) ErrorTrace(w io.Writer, prefix, indent string) (err error) {
	return errorTrace(e, new(bytes.Buffer), w, prefix, indent)
}

func (e Multi) ErrorTrace(w io.Writer, prefix, indent string) (err error) {
	b := new(bytes.Buffer)
	for i := range e {
		err = errorTrace(e[i], b, w, prefix, indent)
		if err == nil && (i+1) < len(e) {
			_, err = io.WriteString(w, "\n")
		}
		if err != nil {
			break
		}
	}
	return
}

func errorTrace(e error, b *bytes.Buffer, w io.Writer, prefix, indent string) (err error) {
	b.Reset()
	if prefix != "" {
		b.WriteString(prefix)
	}
	if indent != "" {
		b.WriteString(indent)
	}
	if _, err = w.Write(b.Bytes()); err != nil {
		return
	}
	switch t := e.(type) {
	case nil:
	case *Err:
		_, err = io.WriteString(w, t.Context.String()+t.Error())
		if err == nil && t.Cause != nil {
			if _, err = io.WriteString(w, "\n"); err != nil {
				err = errorTrace(t.Cause, b, w, string(b.Bytes()), indent)
			}
		}
	case Tracer:
		err = t.ErrorTrace(w, string(b.Bytes()), indent)
	default:
		_, err = io.WriteString(w, e.Error())
	}
	return
}

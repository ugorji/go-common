package logging

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"github.com/ugorji/go-ztesting"
)

func TestHandleErr(t *testing.T) {
	w := new(bytes.Buffer)
	fn1 := func() {
		var err error
		defer func() { fmt.Fprintf(w, "-\n") }()
		// defer HandleErr(nil, "s58", &err) //TODO: Fix this line
		defer func() { fmt.Fprintf(w, "-\n") }()
		PopulatePCLevel = WARNING
		if err = fmt.Errorf("ERROR1"); err != nil {
			return
		}
	}
	AddLogger("", FilterByLevel(ALL), NewHandlerWriter(w, "", nil, 0), false)
	fn1()
	//fmt.Printf("%s", w.String())
	s := w.String()
	if !(strings.HasPrefix(s, "-\nWARNING") && strings.HasSuffix(s, "[logging_test.go:21] s58: ERROR1\n-\n")) {
		ztesting.Log(t, "Received unexpected value: %s", s)
		ztesting.Fail(t)
	}
}











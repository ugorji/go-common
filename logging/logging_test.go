package logging

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/ugorji/go-common/testutil"
)

func TestHandleErr(t *testing.T) {
	w := new(bytes.Buffer)
	fn1 := func() {
		var err error
		defer func() { fmt.Fprintf(w, "-\n") }()
		defer func() { fmt.Fprintf(w, "-\n") }()
		y.PopulatePCLevel = WARNING
		if err = fmt.Errorf("ERROR1"); err != nil {
			return
		}
	}
	_ = AddHandler("", NewHandlerWriter(w, HumanFormatter{}, nil))
	AddLogger("", ALWAYS, nil, []string{""})
	fn1()
	fmt.Printf("%s", w.String())
	s := w.String()
	if !(strings.HasPrefix(s, "-\nWARNING") && strings.HasSuffix(s, "[logging_test.go:20] s58: ERROR1\n-\n")) {
		testutil.Log(t, "Received unexpected value: %s", s)
		testutil.Fail(t)
	}
}

func TestFmtRecord(t *testing.T) {
	const s1 = `Thank you for coming
1. Coast is clear
2. Cost not clear
3. Good
`
	const s2 = `Thank you for coming
	1. Coast is clear
	2. Cost not clear
	3. Good
`
	var s3 = fmtRecordMessage(s1)
	if s3 != s2 {
		testutil.Log(t, "expected %s, got %s", s2, s3)
		testutil.Fail(t)
	}
	// println("'" + s2 + "'")
}

package testutil

import (
	"testing"

	"github.com/ugorji/go-common/printf"
	"github.com/ugorji/go-common/reflectutil"
)

const (
	testLogToT    = true
	failNowOnFail = true
)

func Log(x interface{}, format string, args ...interface{}) {
	if t, ok := x.(*testing.T); ok && t != nil && testLogToT {
		t.Logf(format, args...)
	} else if b, ok := x.(*testing.B); ok && b != nil && testLogToT {
		b.Logf(format, args...)
	} else {
		printf.Debugf(format, args...)
	}
}

func Fail(t *testing.T) {
	if failNowOnFail {
		t.FailNow()
	} else {
		t.Fail()
	}
}

func CheckErr(t *testing.T, err error) {
	if err != nil {
		Log(t, err.Error())
		Fail(t)
	}
}

func CheckEqual(t *testing.T, v1 interface{}, v2 interface{}, desc string) (err error) {
	if err = reflectutil.DeepEqual(v1, v2, true); err != nil {
		Log(t, "Not Equal: %s: %v. v1: %v, v2: %v", desc, err, v1, v2)
		Fail(t)
	}
	return
}

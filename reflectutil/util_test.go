package reflectutil

import (
	"reflect"
	"testing"
)

//define all the tests
var testFmts = []testFmt{
	{int(0), false, false},
	{[]interface{}{"a", "b", "c"}, []string(nil), []string{"a", "b", "c"}},
}

type testFmt struct {
	src interface{}
	typ interface{}
	res interface{}
}

func TestCoerce1(t *testing.T) {
	for _, tt := range testFmts {
		val, err := Coerce(tt.src, tt.typ)
		if err != nil {
			t.Errorf("Error testing: %+v. Err: %v", tt, err)
			return
		}
		if !reflect.DeepEqual(val, tt.res) {
			t.Errorf("Not Equal: Expected: %#v, Got: %#v", tt.res, val)
			return
		}
		t.Logf("Success: %#+v", tt)
	}
}

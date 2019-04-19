package combinationgen

import (
	"testing"
	"reflect"
)

func Test1(t *testing.T) {
	//testLogToT = false
	combo := [][]interface{} {
		{ "a1", "a2", "a3", "a4" },
		{ "b1", "b2" },
		{ "c1", "c2", "c3" },
	}
	expected := [][]interface{} {
		{ "a1", "b1", "c1", },
		{ "a1", "b1", "c2", },
		{ "a1", "b1", "c3", },
		{ "a1", "b2", "c1", },
		{ "a1", "b2", "c2", },
		{ "a1", "b2", "c3", },
		{ "a2", "b1", "c1", },
		{ "a2", "b1", "c2", },
		{ "a2", "b1", "c3", },
		{ "a2", "b2", "c1", },
		{ "a2", "b2", "c2", },
		{ "a2", "b2", "c3", },
		{ "a3", "b1", "c1", },
		{ "a3", "b1", "c2", },
		{ "a3", "b1", "c3", },
		{ "a3", "b2", "c1", },
		{ "a3", "b2", "c2", },
		{ "a3", "b2", "c3", },
		{ "a4", "b1", "c1", },
		{ "a4", "b1", "c2", },
		{ "a4", "b1", "c3", },
		{ "a4", "b2", "c1", },
		{ "a4", "b2", "c2", },
		{ "a4", "b2", "c3", },
	}
	vprops := make([]interface{}, 3)
	cg, err := New(vprops, combo)
	if err != nil {
		Log4test(t, "Error Generating New CombinationGen: %v", err)
		Failtest(t)
	}
	
	i := 0
	v1 := make([][]interface{}, 0, 24)	
	for cg.First(); err == nil; err = cg.Next() {
		Log4test(t, "%v", vprops)
		v2 := make([]interface{}, len(vprops))
		copy(v2, vprops)
		v1 = append(v1, v2)
		i++
		//safe to prevent overflow
		if i > (4 * 2 * 3 * 2) {
			break
		}
	}
	if !reflect.DeepEqual(expected, v1) { 
		Log4test(t, "Expected: %v, Retrieved: %v", expected, v1)
		Failtest(t)
	} 
}


package safestore

import (
	"testing"
	"time"
	"reflect"
	"runtime"
	"runtime/debug"
	"math/rand"
	"strings"
	"github.com/ugorji/go-ztesting"
)

var testSafestoreTbl = []struct {
	Key interface{}
	Val interface{}
	UseLock bool
	TTL time.Duration
}{
	{ "ugorji.1", "nwoke.1", true, 0},
	{ "ugorji.2", "nwoke.2", false, 10 * time.Millisecond},
}

func TestSafeStore(t *testing.T) {
	rq := New(false)
	for _, tb := range testSafestoreTbl {
		rq.useLock = tb.UseLock
		var v2, vnil interface{}
		if tb.TTL > 0 {
			rq.Put(tb.Key, tb.Val, tb.TTL)
			v2 = rq.Get(tb.Key)
			time.Sleep(tb.TTL + 10*time.Millisecond)
			if !rq.useLock { 
				rq.Put("0", nil, 0) //force a purge
			}
			runtime.Gosched()
			vnil = rq.Get(tb.Key)
		} else {
			rq.Put(tb.Key, tb.Val, 0)
			v2 = rq.Get(tb.Key)
		}
		if !reflect.DeepEqual(v2, tb.Val) {
			ztesting.Log(t, "Expecting: %v, Retrieved: %v", tb.Val, v2)
			ztesting.Fail(t)
			continue
		}
		if tb.TTL > 0 && vnil != nil {
			ztesting.Log(t, "Expecting value to have expired but retrieved: %v", vnil)
			ztesting.Fail(t)
			continue
		}
	}
	
}

func TestSafeStoreNil(t *testing.T) {
	rq := New(false)
	mcs := make([]interface{}, 0, 4)
	rq.PutAll(mcs, nil, nil)
}

func BenchmarkSafeStore(b *testing.B) {
	// Will Run first for some configured time. Then run benchmark.
	type T0 struct { v int }
	type T1 struct { v float64 }
	rT0 := reflect.TypeOf(T0{})
	rT1 := reflect.TypeOf(T1{})
	s1 := strings.Repeat("A", 32)
	s2 := strings.Repeat("Z", 64)
	var rq [4]*T
	for i := range rq {
		rq[i] = New(true)
	}
	sleeptime := 0 * time.Millisecond
	c := make(chan bool, 1)
topLoop:
	for i, j := 0, float64(0.0); ; i, j = i+1, j+1 {
		if sleeptime > 0 {
			time.Sleep(sleeptime)
		}
		select {
		case <- c:
			break topLoop
		default:
		}
		k, k2 := rand.Int63(), rand.Int63()
		l := k % int64(len(rq))
		m := i % len(rq)
		fn1 := func(sleep2 time.Duration) (exit bool) {
			if rq[l].useLock == false {
				debug.PrintStack()
				return true
			}
			if sleep2 != 0 {
				time.Sleep(sleep2)
			}
			rq[l].Put(rT0, k, 0)
			_ = rq[l].GetAll([]interface{}{rT0, rT1, s1, s2})
			return false
		}
		fn2 := func(sleep2 time.Duration) (exit bool) {
			if rq[l].useLock == false {
				debug.PrintStack()
				return true
			}
			if sleep2 != 0 {
				time.Sleep(sleep2)
			}
			rq[l].PutAll([]interface{}{rT0, rT1, s1, s2}, []interface{}{k, k2, rT0, rT1},
				[]time.Duration{sleeptime * 20, sleeptime * 40, sleeptime * 40, sleeptime * 60} )
			_ = rq[l].Get(rT1)
			return false
		}
		switch m {
		case 0:
			if fn1(0) {
				break topLoop
			}
		case 1:
			if fn2(0) {
				break topLoop
			}
		case 2:
			go func() {
				if fn1(sleeptime) {
					c <- true
				}
			}()
		case 3:
			go func() {
				if fn2(sleeptime) {
					c <- true
				}
			}()
		}
	}
}



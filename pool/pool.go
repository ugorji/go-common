package pool

import (
	"sync"
	"time"

	"github.com/ugorji/go-common/errorutil"
)

type Action uint8

const (
	GET Action = iota
	PUT
	DISPOSE
)

type Fn func(v interface{}, a Action, currentLen int) (interface{}, error)

// T is a simple structure that maintains a pool.
// It offloads the heavy duty of whether a value should be returned
// to the fn function.
// This way, a user can determine when/how objects are returned.
type T struct {
	vs []interface{}
	mu sync.Mutex
	ch chan interface{}
	fn Fn
}

// Creates a new Pool.
//   - fn: callback function called after GET and before PUT.
//         allows user to create new object or modify one.
//   - initCapacity: preloads this many by calling Get and Put that many times.
func New(fn Fn, load, capacity int) (t *T, err error) {
	if capacity < 1 {
		capacity = 1
	}
	if load < 0 {
		load = 0
	} else if load > capacity {
		load = capacity
	}
	t = &T{
		fn: fn,
		ch: make(chan interface{}, capacity),
	}

	vs := make([]interface{}, load)

	for i := 0; i < load; i++ {
		if vs[i], err = t.Get(0); err != nil {
			return nil, err
		}
	}
	for i := 0; i < load; i++ {
		t.Put(vs[i])
	}
	return t, nil
}

func Must(v interface{}, err error) interface{} {
	if err != nil {
		panic(err)
	}
	return v
}

func (p *T) Get(t time.Duration) (v interface{}, err error) {
	select {
	case v = <-p.ch:
	case <-time.After(t):
	}
	return p.fn(v, GET, len(p.ch))
}

func (p *T) Put(v interface{}) (err error) {
	if v, err = p.fn(v, PUT, len(p.vs)); err != nil || v == nil {
		return
	}
	select {
	case p.ch <- v:
	default:
	}
	return
}

func (p *T) Drain() error {
	var merr errorutil.Multi
LOOP:
	for len(p.ch) > 0 {
		select {
		case v := <-p.ch:
			if _, err := p.fn(v, DISPOSE, 0); err != nil {
				merr = append(merr, err)
			}
		default:
			break LOOP
		}
	}
	return merr.NonNilError()
}

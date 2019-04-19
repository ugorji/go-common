package safestore

import (
	"sync"
	"time"
)

var safestoreSeq uint64 = 0

// I is an interface for a map with expiry support.
type I interface {
	Put(key interface{}, val interface{}, ttl time.Duration)
	Puts(items ...*Item)
	Removes(keys ...interface{})
	Get(key interface{}) interface{}
	Gets(keys ...interface{}) (v []interface{})
}

//var Global *T = New(true)

/*
The T can be used as a thread-safe map of anything to anything.
Use it as a request-scoped cache, etc

Sample Uses:
 - Long-lived shared Cache store
 - Short-lived Request-Scoped Attribute Store
 ...

A T can be lock enabled or not. If you do not use any goroutines within
your code, then create a lock-less T. Purging within a lock-less T
is best effort (we don't synchronize anything).

We also support using safestore as a cache.
  - You put in items along with a timestamp and expiry time
  - A goroutine runs which will purge expired entries out of the cache

Note that, even when used as a cache with a TTL, Get will always return the actual
value stored.

*/
type T struct {
	//id uint64        //added for debugging
	container        map[interface{}]interface{}
	lock             sync.RWMutex
	useLock          bool
	tickerIntervalNs int64
	ticker           *time.Ticker
	tickerC          <-chan time.Time
	tickerResetC     chan bool
	tickerOnce       sync.Once
	lastPurgeNs      int64 // set to -1 after each purge.
	tickerChanges    []int64
}

type safestoreItem struct {
	value        interface{}
	loadTimeNs   int64
	removeTimeNs int64
}

type Item struct {
	Key   interface{}
	Value interface{}
	TTL   time.Duration
}

func New(useLock bool) *T {
	a := &T{
		//id: atomic.AddUint64(&safestoreSeq, 1),
		container: make(map[interface{}]interface{}),
		useLock:   useLock,
	}
	return a
}

func (rq *T) purgeLoopFn() {
	rq.tickerResetC = make(chan bool, 1) //must have buffer of 1 (so someone gets a message in)
	go rq.purgeLoop()
}

func (rq *T) Get(key interface{}) (v interface{}) {
	if rq.useLock {
		rq.lock.RLock()
		defer rq.lock.RUnlock()
	}
	return rq.get(key)
}

func (rq *T) Gets(key ...interface{}) (v []interface{}) {
	if len(key) == 0 {
		return
	}
	if rq.useLock {
		rq.lock.RLock()
		defer rq.lock.RUnlock()
	}
	v = make([]interface{}, len(key))
	for i := 0; i < len(key); i++ {
		// fmt.Printf(">>>%v, %T\n", key[i], key[i])
		v[i] = rq.get(key[i])
	}
	return
}

func (rq *T) GetAll() (v [][2]interface{}) {
	if rq.useLock {
		rq.lock.RLock()
		defer rq.lock.RUnlock()
	}
	v = make([][2]interface{}, len(rq.container))
	i := 0
	for k, _ := range rq.container {
		v[i] = [2]interface{}{k, rq.get(k)}
		i++
	}
	return
}

func (rq *T) Gets2(items ...*Item) {
	ikeys := storedKeys(items)
	vals := rq.Gets(ikeys...)
	for i := 0; i < len(ikeys); i++ {
		items[i].Value = vals[i]
	}
}

func (rq *T) get(key interface{}) (v interface{}) {
	if key == nil {
		return
	}
	if val, ok := rq.container[key]; ok {
		if tt, ok := val.(*safestoreItem); ok {
			val = tt.value
		}
		v = val
	}
	return
}

func (rq *T) put(key interface{}, val interface{}, ttlNs int64) {
	if val == nil {
		delete(rq.container, key)
	} else {
		if ttlNs > 0 {
			ts := time.Now()
			i := &safestoreItem{val, ts.UnixNano(), ts.UnixNano() + ttlNs}
			rq.container[key] = i
		} else {
			rq.container[key] = val
		}
	}
}

func (rq *T) Put(key interface{}, val interface{}, ttl time.Duration) {
	if rq.useLock {
		rq.lock.Lock()
		defer rq.lock.Unlock()
	}
	ttlNs := int64(ttl)
	rq.puts(&Item{key, val, ttl})
	if val != nil && ttlNs > 0 && (rq.tickerIntervalNs == 0 || ttlNs < rq.tickerIntervalNs) {
		rq.resetTicker(ttlNs)
	}
	rq.noLockPurge()
}

func (rq *T) puts(items ...*Item) {
	if len(items) == 0 {
		return
	}
	var minTTLNs int64
	for i := 0; i < len(items); i++ {
		rq.put(items[i].Key, items[i].Value, int64(items[i].TTL))
		if t := int64(items[i].TTL); t > 0 && (minTTLNs == 0 || t < minTTLNs) {
			minTTLNs = t
		}
	}
	if minTTLNs > 0 && (rq.tickerIntervalNs == 0 || minTTLNs < rq.tickerIntervalNs) {
		rq.resetTicker(minTTLNs)
	}
	rq.noLockPurge()
}

func (rq *T) Removes(key ...interface{}) {
	items := make([]*Item, len(key))
	for i, v := range key {
		items[i] = &Item{v, nil, 0}
	}
	rq.Puts(items...)
}

func (rq *T) Puts(items ...*Item) {
	if rq.useLock {
		rq.lock.Lock()
		defer rq.lock.Unlock()
	}
	rq.puts(items...)
}

func (rq *T) Incr(key interface{}, delta int64, initVal uint64) (newval uint64) {
	if rq.useLock {
		rq.lock.Lock()
		defer rq.lock.Unlock()
	}
	if v, ok := rq.get(key).(*uint64); ok {
		newval = uint64(int64(*v) + delta)
		*v = newval
	} else {
		newval = uint64(int64(initVal) + delta)
		rq.put(key, &newval, 0)
	}
	return
}

func (rq *T) purge(ts int64, lock bool) {
	if lock {
		rq.lock.Lock()
		defer rq.lock.Unlock()
	}
	for k, v := range rq.container {
		switch tt := v.(type) {
		case *safestoreItem:
			if ts >= tt.removeTimeNs {
				delete(rq.container, k)
			}
		}
	}
}

func (rq *T) resetTicker(ttlNs int64) {
	if rq.useLock {
		//rq.tickerOnce.Do(rq.purgeLoopFn) //TODO: Go 1.1 Method Values will work here
		rq.tickerOnce.Do(func() { rq.purgeLoopFn() })
		rq.tickerChanges = append(rq.tickerChanges, ttlNs)
		//best effort. Do not hang up trying to send this.
		//Only one person needs to send a notification.
		select {
		case rq.tickerResetC <- true:
		default:
		}
	} else {
		rq.tickerIntervalNs = ttlNs
	}
}

func (rq *T) updateTicker() {
	if !rq.useLock || len(rq.tickerChanges) == 0 {
		return
	}
	rq.lock.Lock()
	defer rq.lock.Unlock()
	var ttlNs int64
	for i := range rq.tickerChanges {
		if rq.tickerChanges[i] > ttlNs {
			ttlNs = rq.tickerChanges[i]
		}
	}
	rq.tickerChanges = rq.tickerChanges[0:0]
	if ttlNs > 0 && rq.tickerIntervalNs != ttlNs {
		if rq.ticker != nil {
			rq.ticker.Stop()
		}
		rq.ticker = time.NewTicker(time.Duration(ttlNs))
		rq.tickerC = rq.ticker.C
		rq.tickerIntervalNs = ttlNs
	}
}

func (rq *T) purgeLoop() {
	for {
		select {
		case tt, ok := <-rq.tickerC:
			if ok && !tt.IsZero() {
				rq.purge(tt.UnixNano(), rq.useLock)
			}
		case <-rq.tickerResetC:
			rq.updateTicker()
		}
	}
}

func (rq *T) noLockPurge() {
	if rq.useLock {
		return
	}
	if tnow := time.Now().UnixNano(); (tnow - rq.lastPurgeNs) > rq.tickerIntervalNs {
		rq.purge(tnow, false)
	}
}

func storedKeys(items []*Item) (ikeys []interface{}) {
	ikeys = make([]interface{}, len(items))
	for i := 0; i < len(ikeys); i++ {
		ikeys[i] = items[i].Key
	}
	// fmt.Printf(">>>>>> storedKeys: ikeys: %v\n", ikeys)
	return
}

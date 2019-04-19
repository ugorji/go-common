package db

// Note:
//   - Load methods must not use variadic, since the return value may be got from
//     the cache's returned value is not always what is passed into the function.

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ugorji/go-common/app"
	"github.com/ugorji/go-common/logging"
	"github.com/ugorji/go-common/safestore"
	"github.com/ugorji/go-common/util"
	"github.com/ugorji/go-common/zerror"
)

type EntityNotFoundError string

func (e EntityNotFoundError) Error() string {
	return string(e)
}

const (
	EntityCacheKeyPfx = "db/db::"
	QueryCacheKeyPfx  = "db/db_query_cache::"

	entityNotFoundMsg = "<App_Entity_Not_Found>"
	//Error returned by Load calls where no entity is found
	EntityNotFoundErr = EntityNotFoundError(entityNotFoundMsg)
)

const (
	CacheMiss CacheResult = iota + 1
	CacheNeg
	CacheHit
)

type CodecBytes interface {
	EncodeBytes(out *[]byte, v interface{}) error
	DecodeBytes(in []byte, v interface{}) error
}

type gobCodec struct{}

func (_ gobCodec) EncodeBytes(out *[]byte, v interface{}) (err error) {
	buf := new(bytes.Buffer)
	if err = gob.NewEncoder(buf).Encode(v); err != nil {
		return
	}
	*out = buf.Bytes()
	return
}

func (_ gobCodec) DecodeBytes(in []byte, v interface{}) error {
	return gob.NewDecoder(bytes.NewBuffer(in)).Decode(v)
}

// var GobCodec = gobCodec{}

// Variable which sets up the Codec for the datastore package.
// By default, it is initialized to use gob.
var Codec CodecBytes = gobCodec{}

type Property struct {
	Name     string
	Value    interface{}
	NoIndex  bool
	Multiple bool
}

type PropertyList []Property

type CacheResult int

type PostLoadHooker interface {
	PostLoadHook() error
}

type PreSaveHooker interface {
	PreSaveHook() error
}

type PostSaveHooker interface {
	PostSaveHook() error
}

type DatastoreKeyAware interface {
	FromDatastoreKey(ctx app.Context, key app.Key) error
	DatastoreKey(ctx app.Context) (app.Key, error)
}

//Holds information about the struct
type TypeMeta struct {
	Type                 reflect.Type
	ParentKeyField       string
	ParentKind           string
	ParentShape          string
	ParentShapeField     string
	KeyField             string
	Kind                 string
	KindId               uint8
	Shape                string
	ShapeId              uint8
	ShapeField           string
	Pinned               bool
	AutoHooks            bool
	UseRequestCache      bool
	UseInstanceCache     bool
	UseSharedCache       bool
	UseDatastore         bool
	SharedCacheTimeout   time.Duration
	InstanceCacheTimeout time.Duration
	DbFields             map[string]*DbFieldMeta
}

//Holds information about a struct field
type DbFieldMeta struct {
	Store       []FieldValueChecker
	Index       []FieldValueChecker
	Type        FieldType
	FieldName   string
	DbName      string
	ReflectType reflect.Type
}

//holds info about datastore key in a struct (for use by cache methods, etc)
type dkeyInfo struct {
	k   app.Key
	enc string
	i   int
}

type FieldValueChecker int32
type FieldType int32

func (d *dkeyInfo) String() string {
	return fmt.Sprintf("%v--%v--%v", d.i, d.k, d.enc)
	//return d.k.String() //can't handle it if d.k is nil
}

//Queries should *typically* call this to postLoad
func PostLoadNoCaching(ctx app.Context, key app.Key, dst interface{}) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	if _, err = FromDatastoreKey(ctx, dst, key); err != nil {
		return err
	}
	if d2, ok := dst.(PostLoadHooker); ok {
		if err = d2.PostLoadHook(); err != nil {
			return err
		}
	}
	return nil
}

func PostLoad(ctx app.Context, useCache bool, keys []app.Key, dst []interface{}) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	for i := 0; i < len(dst); i++ {
		if err = PostLoadNoCaching(ctx, keys[i], dst[i]); err != nil {
			return
		}
	}
	if useCache {
		if err = CachePut(ctx, keys, dst); err != nil {
			return
		}
	}
	return
}

func PreSave(ctx app.Context, useCache bool, keys []app.Key, dst []interface{}) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	for _, d := range dst {
		if d2, ok := d.(PreSaveHooker); ok {
			if err = d2.PreSaveHook(); err != nil {
				return
			}
		}
	}
	return
}

func PostSave(ctx app.Context, keys []app.Key, dst []interface{}) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	//logging.Trace(nil, "Num Keys: %v, Num Objects: %v", len(keys), len(dst))
	for i := 0; i < len(dst); i++ {
		d := dst[i]
		//logging.Trace(nil, "BEFORE SAVE: %T %+v", d, d)
		if _, err = FromDatastoreKey(ctx, d, keys[i]); err != nil {
			return err
		}
		if d2, ok := d.(PostSaveHooker); ok {
			if err = d2.PostSaveHook(); err != nil {
				return err
			}
		}
		//logging.Trace(nil, "AFTER SAVE: %T %+v", d, d)
	}
	//if useCache {
	//	err = CachePut(ctx, keys, dst)
	//} else {
	//	err = CacheDelete(ctx, keys...)
	//}
	//After a Save, always remove from the caches
	err = CacheDelete(ctx, keys...)
	return err
}

//Note: A different entity may be returned e.g. if got from a cache.
//It's okay to pass nil dst to this function. We will return the
//appropriate interface{}
func Get(ctx app.Context, useCache bool, key app.Key, dst interface{}) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	dsts := []interface{}{dst}
	if err = Gets(ctx, useCache, []app.Key{key}, dsts); err != nil {
		if errs, ok := err.(zerror.Multi); ok {
			err = errs[0]
		}
	}
	return
}

//This call supports passing nil members of the dst slice.
//If nil, we will use an new instance of the type appropriate for the Key.
func Gets(ctx app.Context, useCache bool, keys []app.Key, dst []interface{}) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	//error checking first
	//ensure same number of keys and dst
	if len(keys) == 0 || len(keys) != len(dst) {
		err = fmt.Errorf("Gets: lengths: 0 or mismatch; keys: %v, entities: %v",
			len(keys), len(dst))
		return
	}
	dr := app.AppDriver(ctx.AppUUID())
	merr := make(zerror.Multi, len(keys))
	//ensure all dst is set to something (non-nil)
	var kKind, kShape string
	for i := 0; i < len(dst); i++ {
		if dst[i] == nil {
			if kKind, kShape, _, err = dr.GetInfoFromKey(ctx, keys[i]); err != nil {
				return
			}
			tmk := GetLoadedStructMetaFromKind(kKind, kShape)
			dst[i] = reflect.New(tmk.Type).Interface()
		}
	}
	dkeys := keys
	ddst := dst
	dkeymisses := make([]app.Key, 0, 4)
	//retmisses := make([]int, 0, 4)
	//if useCache, check the cache
	if useCache {
		dkeys = make([]app.Key, 0, 4)
		ddst = make([]interface{}, 0, 4)
		cresults, err2 := CacheGet(ctx, keys, dst)
		logging.Trace(ctx, "CacheGet: keys: %v, dst: %v, result: %v, err: %v",
			keys, util.ValuePrintfer{dst}, cresults, err2)
		//println("==============================")
		//debug.PrintStack()
		//if len(misses) == 0 && len(dst) == 1 {
		//	logging.Trace(ctx, "==> %#v", dst[0])
		//}
		//err = nil
		for i := 0; i < len(cresults); i++ {
			switch cresults[i] {
			case CacheNeg:
				merr[i] = EntityNotFoundError(fmt.Sprintf("<Not_Found_In_Cache> Key: %v", keys[i]))
				//merr[i] = util.ErrEntityNotFound
			case CacheMiss:
				var tm *TypeMeta
				tm, err = GetStructMeta(dst[i])
				if err != nil {
					return
				}
				if tm.UseDatastore {
					dkeys = append(dkeys, keys[i])
					ddst = append(ddst, dst[i])
				} else {
					dkeymisses = append(dkeymisses, keys[i])
				}
			}
		}
	}
	if len(dkeys) > 0 {
		logging.Trace(ctx, "After Checking Cache: Will Check datastore for keys: %v", dkeys)
		dstpass := ddst
		dkeyspass := dkeys
		errds := dr.DatastoreGet(ctx, dkeys, ddst)
		logging.Trace(ctx, "Error from DatastoreGet: %v, ddst: %v", errds, util.ValuePrintfer{ddst})
		if errds == nil {
		} else if errs, ok := zerror.Base(errds).(zerror.Multi); ok {
			// look for datastore misses
			dstpass = make([]interface{}, 0, 4)
			dkeyspass = make([]app.Key, 0, 4)
			for i := 0; i < len(errs); i++ {
				if errs[i] == nil {
				} else if IsNotFoundError(errs[i]) {
					logging.Trace(ctx, "Missed in %v: Datastore Key: %v", "Datastore", dkeys[i])
					dkeymisses = append(dkeymisses, dkeys[i])
					for j := 0; j < len(keys); j++ {
						//if the 2 keys are exactly the same
						//FIXME: Assumes that Keys can be checked for equality (e.g. pointers, structs, etc)
						if keys[j] == dkeys[i] {
							merr[j] = errs[i]
							//merr[j] = util.ErrEntityNotFound
							break
						}
					}
				} else {
					//can't handle one of the errors, so return all of them.
					err = fmt.Errorf("Error at index: %v: %v, from MultiError: %v",
						i, errs[i], errds)
					return
				}
			}
		} else {
			err = errds
			return
		}
		if err = PostLoad(ctx, useCache, dkeyspass, dstpass); err != nil {
			return
		}
	}
	// store all misses as negatives in the cache
	if useCache {
		ldkm := len(dkeymisses)
		if ldkm > 0 {
			logging.Trace(ctx, "Storing misses (negatives): %v", dkeymisses)
			dstmisses := make([]interface{}, ldkm)
			for i := 0; i < ldkm; i++ {
				dstmisses[i] = false
			}
			if err = CachePut(ctx, dkeymisses, dstmisses); err != nil {
				return
			}
		}
	}
	//if len(retmisses) == 0 && len(dst) == 1 {
	//	logging.Trace(ctx, "XXXX==> %p, %#v", dst[0], dst[0])
	//}
	//Total Misses = cache negatives + datastore misses
	for i := 0; i < len(merr); i++ {
		if merr[i] != nil {
			err = merr
			break
		}
	}
	return
}

func Put(ctx app.Context, useCache bool, key app.Key, dst interface{}) error {
	return Puts(ctx, useCache, []app.Key{key}, []interface{}{dst})
}

func Puts(ctx app.Context, useCache bool, keys []app.Key, dst []interface{}) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	if err = PreSave(ctx, useCache, keys, dst); err != nil {
		return
	}
	//find subset which are datastore enabled
	dkeys := make([]app.Key, 0, 4)
	ddst := make([]interface{}, 0, 4)
	for i := 0; i < len(keys); i++ {
		tm, err := GetStructMeta(dst[i])
		if err != nil {
			return err
		}
		if tm.UseDatastore {
			dkeys = append(dkeys, keys[i])
			ddst = append(ddst, dst[i])
		}
	}
	dr := app.AppDriver(ctx.AppUUID())
	dmaps := make([]interface{}, len(ddst))
	for j := 0; j < len(ddst); j++ {
		dpl1 := make(PropertyList, 0, 4)
		if err = OrmFromIntf(ddst[j], &dpl1, dr.IndexesOnlyInProps()); err != nil {
			return
		}
		dmaps[j] = &dpl1
		logging.Trace(ctx, "XXXXXXXXXX OrmFromIntf: d: %#v, map: %v", ddst[j], dmaps[j])
	}
	//logging.Trace(ctx, "PropertyList: %#v", dmaps)
	logging.Trace(ctx, "PropertyList: %s", util.ValuePrintfer{dmaps})

	dkeys, err = dr.DatastorePut(ctx, dkeys, ddst, dmaps)
	if err != nil {
		return
	}
	if err = PostSave(ctx, dkeys, ddst); err != nil {
		return
	}
	return
}

func Deletes(ctx app.Context, useCache bool, keys ...app.Key) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	dr := app.AppDriver(ctx.AppUUID())
	if err = dr.DatastoreDelete(ctx, keys); err != nil {
		return
	}
	err = CacheDelete(ctx, keys...)
	return
}

func CachePut(ctx app.Context, keys []app.Key, dst []interface{}) (err error) {
	//defer func() { if err != nil { logging.Error(ctx, "%v", err) } }()
	defer zerror.OnErrorf(1, &err, nil)
	mcitems := make([]*safestore.Item, 0, 4)
	//logging.Trace(nil, "CachePut: dst: %#v, tm: %#v", dst, tm)
	dr := app.AppDriver(ctx.AppUUID())
	sharedCache := dr.SharedCache(false)
	sf := ctx.Store()
	var kKind, kShape string
	for i := 0; i < len(dst); i++ {
		logging.Trace(ctx, "CachePut: i: %v/%v, dst: %v, rt: %v",
			i, len(dst), util.ValuePrintfer{dst[i]}, reflect.TypeOf(dst[i]))
		//this may be false (if a negative)
		if kKind, kShape, _, err = dr.GetInfoFromKey(ctx, keys[i]); err != nil {
			return
		}
		tm := GetLoadedStructMetaFromKind(kKind, kShape)
		//tm := StructMetas.Get(StructMetaKinds.Get(keys[i].Kind())).(*TypeMeta)
		//tm, err := GetStructMeta(dst[i])

		logging.Trace(ctx, "CachePut: tm: %v, err: %v", tm, err)
		//logging.Trace(ctx, "CachePut: tm: %v, err: %v", logging.ValuePrinter{tm}, err)
		if err != nil {
			return err
		}
		enckey := dr.EncodeKey(ctx, keys[i])
		enckey2 := EntityCacheKeyPfx + enckey
		if tm.UseRequestCache {
			sf.Put(enckey, dst[i], 0)
		}
		if tm.UseInstanceCache || (tm.UseSharedCache && sharedCache == nil) {
			dr.InstanceCache().CachePut(ctx,
				&safestore.Item{Key: enckey2, Value: dst[i], TTL: tm.InstanceCacheTimeout})
		}
		if tm.UseSharedCache && sharedCache != nil {
			item := &safestore.Item{Key: enckey2, Value: dst[i]}
			if kKind, kShape, _, err = dr.GetInfoFromKey(ctx, keys[i]); err != nil {
				return
			}
			tm := GetLoadedStructMetaFromKind(kKind, kShape)
			item.TTL = tm.SharedCacheTimeout
			mcitems = append(mcitems, item)
		}
	}
	logging.Trace(ctx, "Attempting to insert in SharedCache: %v items", len(mcitems))
	if len(mcitems) > 0 && sharedCache != nil {
		if err = sharedCache.CachePut(ctx, mcitems...); err != nil {
			return
		}
	}
	return
}

// Does a CacheGet.
func CacheGet(ctx app.Context, keys []app.Key, dst []interface{}) (result []CacheResult, err error) {
	defer zerror.OnErrorf(1, &err, nil)
	logging.Trace(ctx, "Start of CacheGet")
	//logging.Trace(nil, "CacheGet called")
	if len(keys) == 0 || len(keys) != len(dst) {
		err = fmt.Errorf("Gets: lengths: 0 or mismatch; keys: %v, entities: %v", len(keys), len(dst))
		return
	}
	fnHit := func(i int, v interface{}) {
		//dst[i] = v //Set directly
		reflect.ValueOf(dst[i]).Elem().Set(reflect.ValueOf(v).Elem())
		result[i] = CacheHit
	}
	dr := app.AppDriver(ctx.AppUUID())
	sf := ctx.Store()
	result = make([]CacheResult, len(keys))
	for i := 0; i < len(result); i++ {
		result[i] = CacheMiss
	}
	mcck := make([]*safestore.Item, 0, 4) // misses as SharedCache keys
	mccki := make([]*dkeyInfo, 0, 4)
	//do not use a map, since the same key may be in request multiple times
	//mcckm := make(map[string]Key) //
	//mccki := make(map[Key]int)

	sharedCache := dr.SharedCache(false)
	for i := 0; i < len(keys); i++ {
		enckey := dr.EncodeKey(ctx, keys[i])
		enckey2 := EntityCacheKeyPfx + enckey
		var tm *TypeMeta
		if tm, err = GetStructMeta(dst[i]); err != nil {
			return
		}
		if tm.UseRequestCache {
			val := sf.Get(enckey)
			if val != nil {
				if _, ok := val.(bool); ok {
					logging.Trace(ctx, "Miss Recorded Found in %v: Datastore Key: %v",
						"RequestCache", keys[i])
					result[i] = CacheNeg
				} else {
					logging.Trace(ctx, "Found in %v: Datastore Key: %v, Value: %v",
						"RequestCache", keys[i], val)
					fnHit(i, val)
				}
				continue
			}
		}
		if tm.UseInstanceCache || (tm.UseSharedCache && sharedCache == nil) {
			it := &safestore.Item{Key: enckey2}
			if _ = dr.InstanceCache().CacheGet(ctx, it); it.Value != nil {
				if _, ok := it.Value.(bool); ok {
					result[i] = CacheNeg
					logging.Trace(ctx, "Miss Recorded Found in %v: Datastore Key: %v",
						"InstanceCache", keys[i])
				} else {
					logging.Trace(ctx, "Found in %v: Datastore Key: %v, Value: %#v",
						"InstanceCache", keys[i], it.Value)
					fnHit(i, it.Value)
				}
				continue
			}
		}
		if tm.UseSharedCache && sharedCache != nil {
			mcck = append(mcck, &safestore.Item{Key: enckey2, Value: dst[i]})
			mccki = append(mccki, &dkeyInfo{keys[i], enckey2, i})
			//mcckm[enckey2] = keys[i]
			//mccki[keys[i]] = i
		}
	}

	if len(mcck) > 0 && sharedCache != nil {
		logging.Trace(ctx, "Checking SharedCache for: %v", mccki)
		err2 := sharedCache.CacheGet(ctx, mcck...)
		logging.Trace(ctx, "After SharedCache GetMulti. err: %v", err2)
		if err2 != nil {
			err = err2
			return
		}
		for i, dki := range mccki {
			v := mcck[i].Value
			logging.Trace(ctx, "----- SharedCache Result: %#v", v)
			if v != nil {
				if _, ok2 := v.(bool); ok2 {
					logging.Trace(ctx, "Miss Recorded Found in SharedCache: "+
						"Datastore Key: %v, Value: %v", dki.k, v)
					result[dki.i] = CacheNeg
				} else {
					logging.Trace(ctx, "SharedCache Unmarshal: %v, %#v", dki.i, dst[dki.i])
					logging.Trace(ctx, "Found in SharedCache: Datastore Key: %v, Value: %v", dki.k, v)
					fnHit(dki.i, v)
				}
			}
		}
	}

	logging.Trace(ctx, "Cache Results: %v", result)
	return
}

func CacheDelete(ctx app.Context, keys ...app.Key) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	dr := app.AppDriver(ctx.AppUUID())
	sf := ctx.Store()
	mcs0 := make([]interface{}, 0, 4)
	mcs := make([]interface{}, 0, 4)
	for i := 0; i < len(keys); i++ {
		enckey := dr.EncodeKey(ctx, keys[i])
		mcs0 = append(mcs0, enckey)
		enckey2 := EntityCacheKeyPfx + enckey
		mcs = append(mcs, enckey2)
	}
	sf.Removes(mcs0...)
	dr.InstanceCache().CacheDelete(ctx, mcs0...)
	if sharedCache := dr.SharedCache(false); sharedCache != nil {
		err = sharedCache.CacheDelete(ctx, mcs...)
	}
	return
}

//Populate a struct from a datastore key
func FromDatastoreKey(ctx app.Context, d interface{}, key app.Key) (d2 interface{}, err error) {
	defer zerror.OnErrorf(1, &err, nil)
	var tm *TypeMeta
	var s reflect.Value
	dr := app.AppDriver(ctx.AppUUID())
	if d == nil {
		kind, shape, _, err2 := dr.GetInfoFromKey(ctx, key)
		if err2 != nil {
			err = err2
			return
		}
		tm = GetLoadedStructMetaFromKind(kind, shape)
		if tm == nil {
			err = fmt.Errorf("No TypeMeta found for kind: %v, shape: %v", kind, shape)
			return
		}
		s = reflect.New(tm.Type)
		d = s.Interface()
		s = s.Elem()
	}
	d2 = d
	if dsw, ok := d.(DatastoreKeyAware); ok {
		err = dsw.FromDatastoreKey(ctx, key)
		return
	}
	//logging.BIOutput.Write(debug.Stack())
	//util.PrintStackFrames(logging.BIOutput, 3)
	//logging.Trace(nil, "FromDatastoreKey: d: %#v", d)
	if tm == nil {
		if tm, err = GetStructMeta(d); err != nil {
			return
		}
	}
	if !s.IsValid() {
		s = reflect.ValueOf(d).Elem()
	}
	//s := reflect.ValueOf(d)
	//if s.Kind() != reflect.Struct { s = s.Elem() }

	//set Shape field to value of Shape if it is not yet set (ie if it is "")
	if tm.Shape != "" && tm.ShapeField != "" {
		fshape := s.FieldByName(tm.ShapeField)
		if fshape.IsValid() && fshape.String() == "" {
			fshape.SetString(tm.Shape)
		}
	}
	if tm.ParentShape != "" && tm.ParentShapeField != "" {
		fshape := s.FieldByName(tm.ParentShapeField)
		if fshape.IsValid() && fshape.String() == "" {
			fshape.SetString(tm.ParentShape)
		}
	}
	if err = fromDskey(ctx, s, tm.KeyField, tm.Shape, key); err != nil {
		return
	}
	if kpar := dr.ParentKey(ctx, key); kpar != nil {
		if err = fromDskey(ctx, s, tm.ParentKeyField, tm.ParentShape, kpar); err != nil {
			return
		}
	}
	return
}

// Get a datastore key from a struct.
// If the Id of the passed interface (d) is < 0, the returned key will have a fresh id allocated
// from the backend datastore.
func DatastoreKey(ctx app.Context, d interface{}) (k app.Key, err error) {
	// defer func() {
	// 	logging.Error(nil, ">>>>> last %v %T\n%s\n", err, err, util.Stack(nil, false))
	// }()
	defer zerror.OnErrorf(1, &err, nil)
	// defer func() {
	// 	logging.Error(nil, ">>>>> before last %v %T\n%s\n", err, err, util.Stack(nil, false))
	// }()
	if dsw, ok := d.(DatastoreKeyAware); ok {
		return dsw.DatastoreKey(ctx)
	}
	tm, err := GetStructMeta(d)
	if err != nil {
		return
	}
	s := reflect.ValueOf(d).Elem()
	//if s.Kind() != reflect.Struct { s = s.Elem() }
	var pk app.Key
	if tm.ParentKeyField != "" {
		pk, err = dskey(ctx, s, tm.ParentKeyField, tm.ParentShapeField,
			tm.ParentKind, tm.ParentShape, nil)
		if err != nil {
			return
		}
		if pk.Incomplete() {
			err = fmt.Errorf("DatastoreKey: Error: Parent Key is not in datastore. Id: %v", pk.EntityId())
			//logging.PrintStack()
			return
		}
	}
	return dskey(ctx, s, tm.KeyField, tm.ShapeField, tm.Kind, tm.Shape, pk)
}

//This will return a Key.
//It also respects shape
//  - write shape information to the Key if not in there already
//  - write shape information to the ShapeField if defined and not set
func dskey(ctx app.Context, s reflect.Value,
	keyfield string, shapefield string, kind string, shape string, pkey app.Key,
) (k app.Key, err error) {
	defer zerror.OnErrorf(1, &err, nil)
	dr := app.AppDriver(ctx.AppUUID())
	intId := s.FieldByName(keyfield).Int()
	if shape != "" && shapefield != "" {
		fshape := s.FieldByName(shapefield)
		if fshape.IsValid() && fshape.String() == "" {
			fshape.SetString(shape)
		}
	}
	if k, err = dr.NewKey(ctx, kind, shape, intId, pkey); err != nil {
		return
	}
	if !k.Incomplete() && intId <= 0 {
		s.FieldByName(keyfield).SetInt(k.EntityId())
	}
	return
}

func fromDskey(ctx app.Context, s reflect.Value, field string, shape string, key app.Key) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	f := s.FieldByName(field)
	if !f.IsValid() {
		return fmt.Errorf("No Field Named: %v found in %v", field, s.Interface())
	}
	//logging.Trace(nil, "fromDskey: field: %v, val: %#v, fieldval: %#v, valid: %v, " +
	//"settable: %v, key: %v", field, s.Interface(), f.Interface(), f.IsValid(), f.CanSet(), key)
	dr := app.AppDriver(ctx.AppUUID())
	_, _, kid, err := dr.GetInfoFromKey(ctx, key)
	if err != nil {
		return
	}
	f.SetInt(kid) //f.Set(reflect.ValueOf(key.IntID()))
	return
}

//Decode a cache entry. It knows that Negatives are recorded as 'false',
//supports doing a quick check for false.
func CacheEntryDecode(bs []byte, v interface{}) (v2 interface{}, err error) {
	defer zerror.OnErrorf(1, &err, nil)
	if len(bs) == 0 {
		return
	}
	// "", false, true, etc are typically encoded as 4 bytes or less. Safe'ish check.
	if len(bs) <= 4 {
		var bv bool
		if err2 := Codec.DecodeBytes(bs, &bv); err2 == nil {
			v2 = bv
			return
		}
	}
	if v2 == nil {
		if err = Codec.DecodeBytes(bs, v); err != nil {
			return
		}
		v2 = v
	}
	return
}

//Query the datastore.
//This supports using the InstanceCache as a Query Cache.
//Ensure only entities of this kind and shape are returned.
func QuerySupport(ctx app.Context, qString string, kind string,
	shape string, nextFn func() (k app.Key, cursor string, err error),
) (res []app.Key, lastqcur string, err error) {
	defer zerror.OnErrorf(1, &err, nil)
	//var obj interface{}
	tm := GetLoadedStructMetaFromKind(kind, shape)
	if tm == nil {
		err = fmt.Errorf("No TypeMeta found for kind: %v, shape: %v", kind, shape)
		return
	}
	res = make([]app.Key, 0, 4)
	var kKind, kShape string
	//Do caching if instance caching supported.
	useQCache, cacheTimeout := tm.UseInstanceCache, tm.InstanceCacheTimeout
	logging.Trace(ctx, "QuerySupport: Will Check Query Cache for kind: %v, shape: %v", kind, shape)
	dr := app.AppDriver(ctx.AppUUID())
	if useQCache {
		it := &safestore.Item{Key: QueryCacheKeyPfx + qString}
		dr.InstanceCache().CacheGet(ctx, it)
		if it.Value != nil {
			logging.Trace(ctx, "QuerySupport: Found results in cache")
			for _, key := range it.Value.([]app.Key) {
				kKind, kShape, _, err = dr.GetInfoFromKey(ctx, key)
				if err != nil {
					return
				}
				if kind != "" && kind != kKind {
					continue
				}
				if shape != "" && shape != kShape {
					continue
				} //skip keys with different shape
				res = append(res, key)
			}
			logging.Trace(ctx, "QuerySupport: Returning results from cache: %v", res)
			return
		}
	}
	logging.Trace(ctx, "QuerySupport: Not In Query Cache. Will look at backend query results")
	//Not in query cache. So continue.
	var cachedKeys []app.Key
	//if processcache, store query in cache again (for maybe later)
	if useQCache {
		cachedKeys = make([]app.Key, 0, 4)
	}

	var key app.Key
	for {
		//logging.Trace(nil, "GAE DRIVER: %#v", obj)
		key, lastqcur, err = nextFn()
		if zerror.Base(err) == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}
		if kKind, kShape, _, err = dr.GetInfoFromKey(ctx, key); err != nil {
			return
		}
		logging.Trace(ctx, "QuerySupport: Will check key: %v, having kind: %v, Shape: %v", key, kKind, kShape)
		if kind != "" && kind != kKind {
			continue
		}
		if shape != "" && shape != kShape {
			continue
		} //skip keys with different shape

		if useQCache {
			cachedKeys = append(cachedKeys, key)
		}
		res = append(res, key)
	}
	if useQCache {
		it := &safestore.Item{
			Key:   QueryCacheKeyPfx + qString,
			Value: cachedKeys,
			TTL:   cacheTimeout,
		}
		dr.InstanceCache().CachePut(ctx, it)
	}
	logging.Trace(ctx, "QuerySupport: Returning Keys: %v", res)
	return
}

func QueryAsString(parentKey app.Key, kind string, opts *app.QueryOpts, filters ...*app.QueryFilter,
) (qString string) {
	sa := make([]string, 0, 4)
	if parentKey != nil {
		sa = append(sa, fmt.Sprintf("%v", parentKey))
	}
	sa = append(sa, kind)
	if opts != nil {
		sa = append(sa, "====>")
		sa = append(sa, opts.Shape,
			opts.Order, strconv.Itoa(opts.Limit), strconv.Itoa(opts.Offset),
			opts.StartCursor, opts.EndCursor)
	}
	if len(filters) > 0 {
		sa = append(sa, "====>")
		for i, fp0 := range filters {
			sa = append(sa, fmt.Sprintf("%v", fp0))
			if i != len(filters)-1 {
				sa = append(sa, "|")
			}
		}
	}
	qString = strings.Join(sa, "^^")
	return
}

//Note: When loading, do not use variadic, since the returned values may not be
//what is sent variadic'ally.
func Load(ctx app.Context, useCache bool, entities []interface{}) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	//logging.DebugStack(ctx, "")
	dr := app.AppDriver(ctx.AppUUID())
	useCache = dr.UseCache(ctx, useCache)
	keys := make([]app.Key, 0, len(entities))
	for i := 0; i < len(entities); i++ {
		var key app.Key
		if key, err = DatastoreKey(ctx, entities[i]); err != nil {
			return
		}
		if key.Incomplete() {
			err = fmt.Errorf("Attempting to load an entity which is not yet in datastore: %#v", entities[i])
			return
		}
		keys = append(keys, key)
	}
	logging.Trace(ctx, "db.Load: %v/%v entities: %v, keys: %v", len(entities), len(keys),
		util.ValuePrintfer{entities}, util.ValuePrintfer{keys})
	//debug.PrintStack()
	//Call Get on them
	if err = Gets(ctx, useCache, keys, entities); err != nil {
		return
	}
	return
}

func Save(ctx app.Context, entities ...interface{}) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	//Create a corresponding slice of Keys
	//Call Save
	dr := app.AppDriver(ctx.AppUUID())
	keys := make([]app.Key, 0, len(entities))
	var key app.Key
	for i := 0; i < len(entities); i++ {
		if key, err = DatastoreKey(ctx, entities[i]); err != nil {
			return err
		}
		keys = append(keys, key)
	}
	useCache := dr.UseCache(ctx, true)
	return Puts(ctx, useCache, keys, entities)
}

func LoadOne(ctx app.Context, useCache bool, maybeCreate bool, entity interface{}) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	dsts := []interface{}{entity}
	err = Load(ctx, maybeCreate, dsts)
	// TODO: verify that removing MultiFirst call here works
	// if maybeCreate && zerror.IsNotFound(zerror.MultiFirst(err)) {
	if maybeCreate && IsNotFoundError(err) {
		err = Save(ctx, entity)
	}
	return
}

func EntitiesForKeys(ctx app.Context, keys []app.Key, load bool, useCache bool) (res []interface{}, err error) {
	defer zerror.OnErrorf(1, &err, nil)
	res = make([]interface{}, len(keys))
	for i := 0; i < len(keys); i++ {
		if res[i], err = FromDatastoreKey(ctx, nil, keys[i]); err != nil {
			return
		}
	}
	if load {
		err = Load(ctx, useCache, res)
	}
	return
}

func EntityForKey(ctx app.Context, key app.Key, load bool, useCache bool) (res interface{}, err error) {
	defer zerror.OnErrorf(1, &err, nil)
	if res, err = FromDatastoreKey(ctx, nil, key); err != nil {
		return
	}
	if load {
		err = LoadOne(ctx, useCache, false, res)
	}
	return
}

// IsNotFound will return true if this Error is a NotFound error.
// It also returns true if argument is a Multi, and all its errors are NotFound errors.
func IsNotFoundError(err error) (b bool) {
	if err == nil {
		return
	}
	switch tt2 := err.(type) {
	case EntityNotFoundError:
		b = true
	case zerror.Multi:
		if len(tt2) == 0 {
			break
		}
		b = true
		for _, err2 := range tt2 {
			if !IsNotFoundError(err2) {
				b = false
				break
			}
		}
	}
	return
}

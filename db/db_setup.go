package db

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"github.com/ugorji/go-common/logging"

	//"runtime/debug"
	"github.com/ugorji/go-common/safestore"
	"github.com/ugorji/go-common/zerror"
)

const (
	StructInfoField = "_struct"
	StructTagKey    = "db"
	//SynFieldNamePfx = "X__"
)

const (
	ALWAYS_FVC FieldValueChecker = iota + 1
	NOT_ALWAYS_FVC
	EMPTY_FVC
	NOT_EMPTY_FVC
)

const (
	STRUC_FTYPE FieldType = iota + 1
	MARSHAL_FTYPE
	EXPANDO_FTYPE
	ITREE_FTYPE
	REGULAR_FTYPE
)

var (
	StructMetas      = safestore.New(true) //map reflect.Type to *TypeMeta
	StructMetaKinds  = safestore.New(true) //map Kind to reflect.Type
	structMetasMutex sync.Mutex
)

func NewDbFieldMeta() *DbFieldMeta {
	return &DbFieldMeta{
		Store: make([]FieldValueChecker, 0, 2),
		Index: make([]FieldValueChecker, 0, 2),
		Type:  REGULAR_FTYPE,
	}
}

func NewTypeMeta() *TypeMeta {
	return &TypeMeta{
		UseRequestCache:      true,
		UseSharedCache:       true,
		UseDatastore:         true,
		InstanceCacheTimeout: 10 * time.Minute,
		SharedCacheTimeout:   10 * time.Minute,
		DbFields:             make(map[string]*DbFieldMeta),
	}
}

func getStructMetaKindKey(kind string, shape string) string {
	s := kind
	if shape != "" {
		s = kind + ":" + shape
	}
	return s
}

func GetLoadedStructMetaFromKind(kind string, shape string) *TypeMeta {
	rt := GetLoadedReflectTypeFromKind(kind, shape)
	if rt == nil {
		return nil
	}
	return StructMetas.Get(rt).(*TypeMeta)
}

func GetLoadedReflectTypeFromKind(kind string, shape string) reflect.Type {
	smkKey := getStructMetaKindKey(kind, shape)
	return StructMetaKinds.Get(smkKey).(reflect.Type)
}

func GetStructMeta(s interface{}) (tm *TypeMeta, err error) {
	//logging.Trace(nil, "GetStructMeta: %#v", s)
	return GetStructMetaFromType(reflect.TypeOf(s))
}

func GetStructMetaFromType(rt reflect.Type) (tm *TypeMeta, err error) {
	defer zerror.OnErrorf(1, &err, nil)
	//logging.RawOut(" --------------- STANDARD OUT --------------- \n")
	rt00 := rt
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return nil, fmt.Errorf("Only struct or pointer to a strut is supported: %v, orig: %v", rt, rt00)
	}
	//rt = reflect.PtrTo(rt)
	//logging.RawOut(" #### Reflect Type: Of: ((%+v)) is: ((%+v)) with kind: ((%v))\n",
	//      s, rt, rt.Kind())
	//tkey := rt.PkgPath() + ":" + rt.
	structMetasMutex.Lock()
	defer structMetasMutex.Unlock()
	v := StructMetas.Get(rt)
	if v != nil {
		ok := false
		if tm, ok = v.(*TypeMeta); !ok {
			return nil, fmt.Errorf("Type assertion failed getting struct metadata")
		}
	} else {
		//debug.PrintStack()
		logging.Trace(nil, "Creating StructMeta for: %v", rt)
		tm = NewTypeMeta()
		tm.Type = rt
		if err = addStructMetaFromTypeForStructInfo(rt, tm); err != nil {
			return
		}
		if err = addStructMetaFromType(rt, tm); err != nil {
			return
		}
		if tm.Kind != "" {
			smkKey := getStructMetaKindKey(tm.Kind, tm.Shape)
			if tmo := StructMetaKinds.Get(smkKey); tmo != nil {
				return nil, fmt.Errorf("Error registering: %#v for kind: %v, shape: %v. Type: %#v"+
					" already registered.", tm, tm.Kind, tm.Shape, tmo)
			}
			StructMetaKinds.Put(smkKey, rt, 0)
			StructMetas.Put(rt, tm, 0)
		}
		logging.Trace(nil, "Created StructMeta for: %v: %#v", rt, tm)
		//logging.Trace(nil, "Created StructMeta for: %v: %s", rt, util.ValuePrintfer{tm})
	}
	//logging.RawOut("%+v", tm)
	return
}

func bValue(s string) bool {
	bv, err := strconv.ParseBool(s)
	if err != nil {
		bv = false
	}
	return bv
}

func addStructMetaFromTypeForStructInfo(rt reflect.Type, tm *TypeMeta) error {
	sf, ok := rt.FieldByName(StructInfoField)
	if ok {
		if stv := getStructTagValue(sf); stv != nil {
			for i := 0; i < len(stv); i++ {
				switch stv[i] {
				case "auto":
					tm.AutoHooks = true
				case "rc":
					tm.UseRequestCache = true
				case "pc":
					tm.UseInstanceCache = true
				case "mc":
					tm.UseSharedCache = true
				case "ds":
					tm.UseDatastore = true
				case "pinned":
					tm.Pinned = true
				}
				switch {
				case strings.HasPrefix(stv[i], "auto="):
					tm.AutoHooks = bValue(stv[i][5:])
				case strings.HasPrefix(stv[i], "rc="):
					tm.UseRequestCache = bValue(stv[i][3:])
				case strings.HasPrefix(stv[i], "pc="):
					tm.UseInstanceCache = bValue(stv[i][3:])
				case strings.HasPrefix(stv[i], "mc="):
					tm.UseSharedCache = bValue(stv[i][3:])
				case strings.HasPrefix(stv[i], "ds="):
					tm.UseDatastore = bValue(stv[i][3:])
				case strings.HasPrefix(stv[i], "pinned="):
					tm.Pinned = bValue(stv[i][7:])
				case strings.HasPrefix(stv[i], "pcto="):
					//to, err := util.FineTimeSecs(stv[i][5:])
					to, err := time.ParseDuration(stv[i][5:])
					if err != nil {
						return err
					}
					tm.InstanceCacheTimeout = to
				case strings.HasPrefix(stv[i], "mcto="):
					to, err := time.ParseDuration(stv[i][5:])
					if err != nil {
						return err
					}
					tm.SharedCacheTimeout = to
				case strings.HasPrefix(stv[i], "keyf="):
					tm.KeyField = stv[i][5:]
				case strings.HasPrefix(stv[i], "kind="):
					tm.Kind = stv[i][5:]
				case strings.HasPrefix(stv[i], "kindid="):
					to, err := strconv.ParseUint(stv[i][7:], 10, 8)
					if err != nil {
						return err
					}
					tm.KindId = uint8(to)
				case strings.HasPrefix(stv[i], "shapeid="):
					to, err := strconv.ParseUint(stv[i][8:], 10, 8)
					if err != nil {
						return err
					}
					tm.ShapeId = uint8(to)
				case strings.HasPrefix(stv[i], "shape="):
					tm.Shape = stv[i][6:]
				case strings.HasPrefix(stv[i], "shapef="):
					tm.ShapeField = stv[i][7:]
				case strings.HasPrefix(stv[i], "pkeyf="):
					tm.ParentKeyField = stv[i][6:]
				case strings.HasPrefix(stv[i], "pkind="):
					tm.ParentKind = stv[i][6:]
				case strings.HasPrefix(stv[i], "pshape="):
					tm.ParentShape = stv[i][7:]
				case strings.HasPrefix(stv[i], "pshapef="):
					tm.ParentShapeField = stv[i][8:]
				}
			}
		}
	}
	return nil
}

func addStructMetaFromType(rt reflect.Type, tm *TypeMeta) error {
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		if sf.Name == StructInfoField {
			continue
		}
		if sf.Anonymous {
			addStructMetaFromType(sf.Type, tm)
			continue
		}
		if stv := getStructTagValue(sf); stv != nil {
			fm := NewDbFieldMeta()
			fm.ReflectType = sf.Type
			fm.FieldName = sf.Name
			fm.DbName = sf.Name
			for i := 0; i < len(stv); i++ {
				switch {
				case strings.HasPrefix(stv[i], "dbname="):
					fm.DbName = stv[i][7:]
				case strings.HasPrefix(stv[i], "ftype="):
					switch stv[i][6:] {
					case "struc":
						fm.Type = STRUC_FTYPE
					case "marshal":
						fm.Type = MARSHAL_FTYPE
					case "expando":
						fm.Type = EXPANDO_FTYPE
					case "tree":
						fm.Type = ITREE_FTYPE
					}
				case strings.HasPrefix(stv[i], "store="):
					fm.Store = appendFVC(stv[i][6:], fm.Store)
				case strings.HasPrefix(stv[i], "index="):
					fm.Index = appendFVC(stv[i][6:], fm.Index)
				}
			}
			for _, fm2 := range tm.DbFields {
				if fm2.DbName == fm.DbName {
					return fmt.Errorf("Error binding dbname: %v to DBFieldMeta: %#v. Already bound to: %#v",
						fm.DbName, fm, fm2)
				}
			}
			//hack: To ensure we see things in dev server
			//if appengine.IsDevAppServer() { fm.Index = []FieldValueChecker{ALWAYS_FVC} }
			if len(fm.Index) == 0 {
				fm.Index = []FieldValueChecker{NOT_EMPTY_FVC}
			}
			if len(fm.Store) == 0 {
				fm.Store = []FieldValueChecker{NOT_EMPTY_FVC}
			}
			tm.DbFields[sf.Name] = fm
		}
	}
	return nil
}

func appendFVC(s string, fvca []FieldValueChecker) []FieldValueChecker {
	stv2 := strings.Split(s, "|")
	for _, s2 := range stv2 {
		switch s2 {
		case "y":
			fvca = append(fvca, ALWAYS_FVC)
		case "!y":
			fvca = append(fvca, NOT_ALWAYS_FVC)
		case "z":
			fvca = append(fvca, EMPTY_FVC)
		case "!z":
			fvca = append(fvca, NOT_EMPTY_FVC)
		}
	}
	return fvca
}

func getStructTagValue(sf reflect.StructField) []string {
	stag := sf.Tag.Get(StructTagKey)
	var stv []string
	if stag != "" {
		stv = strings.Split(stag, ",")
	}
	return stv
}

package db

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/ugorji/go-common/logging"
	"github.com/ugorji/go-common/tree"
	"github.com/ugorji/go-common/zerror"
)

//Hack: Holds a mapping of basic types (and their kinds) to slices of them,
//so we can do MakeSlice appropriately
//FIXME: Fix if/when they support creating slices given underlying type
var (
	//need to support all the basic types (Type and Kind)
	//that can go into the datastore.
	ormSlices  = make(map[interface{}]reflect.Type)
	invalidRV  = reflect.Value{}
	byteArrTyp = reflect.TypeOf([]byte{})
	//richapp.lobkeyTyp = reflect.TypeOf(richapp.BlobKey(""))
)

//Called by custom applications to register their own custom types,
//so that we can find the slice type for it.
//It binds a Kind, and element type to a slice type.
//  - This way, given a Kind, we can get the slice type.
//  - Also, given an element type, we can get a slice type.
//  - An error is returned if a slice is not passed in.
//Note that we do not override any registrations. For everything to work,
//We expect that this package has already registered all the primitive types.
//ie. (u)intXXX, floatXXX, byte, bool, string, []byte, etc.
//Also, we only register kinds for primitive types:
//  - not for reference kinds like channels, pointers, maps, interface, func, etc
func HackRegisterSliceType(slcs ...interface{}) error {
	for _, slc := range slcs {
		rt1 := reflect.TypeOf(slc)
		if rt1.Kind() != reflect.Slice {
			return fmt.Errorf("Non Slice Parameter: %v", slc)
		}
		rt2 := rt1.Elem()
		rk2 := rt2.Kind()
		ok := false
		//don't override a registration.
		if _, ok = ormSlices[rt2]; !ok {
			ormSlices[rt2] = rt1
		}
		switch rk2 {
		case reflect.Bool, reflect.Uintptr,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Float32, reflect.Float64,
			reflect.Complex64, reflect.Complex128:
			if _, ok = ormSlices[rk2]; !ok {
				ormSlices[rk2] = rt1
			}
		}
	}
	return nil
}

//Populate from a struct
func OrmFromIntf(d interface{}, m *PropertyList, indexesOnly bool) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	tm, err := GetStructMeta(d)
	if err != nil {
		return err
	}
	dv := reflect.ValueOf(d)
	if dv.Kind() != reflect.Struct {
		dv = dv.Elem()
	}
	for sfname, fm := range tm.DbFields {
		fv := dv.FieldByName(sfname)
		var val interface{}
		switch fm.Type {
		case REGULAR_FTYPE:
			if val = fv.Interface(); val != nil && fvc('s', val, fm, tm) {
				addToPropList(m, fm.DbName, val, indexesOnly, fv, false, fm, tm)
			}
		case MARSHAL_FTYPE:
			var val []byte
			logging.Trace(nil, "Calling Encode for: %v", fv)
			err = Codec.EncodeBytes(&val, fv.Interface())
			if err != nil {
				return
			}
			if val != nil && fvc('s', val, fm, tm) {
				addToPropList(m, fm.DbName, val, indexesOnly, invalidRV, false, fm, tm)
			}
		case STRUC_FTYPE:
			if val = fv.Interface(); val != nil && fvc('s', val, fm, tm) {
				if err = ormFromIntfStruc(m, indexesOnly, fv, fm, tm); err != nil {
					return err
				}
			}
		case ITREE_FTYPE:
			if val = fv.Interface(); val != nil && fvc('s', val, fm, tm) {
				itrVal := val.(*tree.Int64Node)
				itrCodec := tree.NewInt64Codec()
				itrSlice := itrCodec.EncodeChildren(itrVal, nil)
				for _, itrslv := range itrSlice {
					addToPropList(m, fm.DbName, itrslv, indexesOnly, invalidRV, true, fm, tm)
				}
			}
		case EXPANDO_FTYPE:
			if val = fv.Interface(); val != nil && fvc('s', val, fm, tm) {
				for _, k := range fv.MapKeys() {
					mykey := fmt.Sprintf("%v_%v", fm.DbName, k.Interface())
					myval := fv.MapIndex(k).Interface()
					addToPropList(m, mykey, myval, indexesOnly, invalidRV, false, fm, tm)
				}
			}
		}
	}
	return nil
}

//Populate to a struct
func OrmToIntf(m *PropertyList, d interface{}) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	tm, err := GetStructMeta(d)
	if err != nil {
		return
	}
	dv := reflect.ValueOf(d)
	if dv.Kind() != reflect.Struct {
		dv = dv.Elem()
	}
	//dv := reflect.ValueOf(d).Elem()
	logging.Trace(nil, " XYXYXYXYXY: m: %v, %#v", m, dv)
	for sfname, fm := range tm.DbFields {
		fv := dv.FieldByName(sfname)
		//var val interface{}
		//logging.Trace(nil, " YYYYY fmType: %v, field: %v ==> %#v, val: %v ==> %#v",
		//	fm.Type, fm.FieldName, fm.ReflectType.Name(), fv.Type(), fv.Interface())
		switch fm.Type {
		case REGULAR_FTYPE:
			ormGetSetVal(m, fm.DbName, fv)
			//if myval, err := ormGetVal(m, fm.DbName, fv); err == nil {
			//	ormSetVal(fv, myval)
			//}
		case MARSHAL_FTYPE:
			for _, val := range *m {
				if val.Name == fm.DbName {
					fsrc := val.Value.([]byte)
					logging.Trace(nil, "Calling Decode for: %v", fsrc)
					err = Codec.DecodeBytes(fsrc, fv)
					if err != nil {
						return
					}
					break
				}
			}
		case ITREE_FTYPE:
			fsrc := make([]int64, 0, 4)
			for _, val := range *m {
				if val.Name == fm.DbName {
					fsrc = append(fsrc, val.Value.(int64))
				}
			}
			if len(fsrc) > 0 {
				itr := new(tree.Int64Node)
				itrCodec := tree.NewInt64Codec()
				itrCodec.DecodeChildren(itr, fsrc)
				fv.Set(reflect.ValueOf(itr))
			}
		case STRUC_FTYPE:
			if err = ormToIntfStruc(m, fv, fm, tm); err != nil {
				return
			}
		case EXPANDO_FTYPE:
			m2 := reflect.MakeMap(fv.Type())
			spfx := fm.DbName + "_"
			spfxlen := len(spfx)
			for _, val := range *m {
				if strings.HasPrefix(val.Name, spfx) {
					m2.SetMapIndex(reflect.ValueOf(val.Name[spfxlen:]), reflect.ValueOf(val.Value))
				}
			}
			fv.Set(m2)
		}
	}
	return nil
}

func ormToIntfStruc(m *PropertyList, fr reflect.Value,
	fm *DbFieldMeta, tm *TypeMeta) error {
	stf, _ := tm.Type.FieldByName(fm.FieldName) //fr.Type()
	rt := stf.Type
	rt2 := rt
	fr2 := fr
	logging.Trace(nil, "####### ormToIntfStruc: rVal: %#v, dsproplist: %#v", fr, *m)
	switch rt.Kind() {
	case reflect.Map:
		mykeyK := fm.DbName + "_k"
		mykeyV := fm.DbName + "_v"
		var mykey interface{}
		mymap := make(map[interface{}]interface{})
		for _, val := range *m {
			if val.Name == mykeyK {
				mykey = val.Value
			} else if val.Name == mykeyV {
				mymap[mykey] = val.Value
				mykey = nil
			}
		}
		flen := len(mymap)
		if flen > 0 {
			fkk := rt.Key().Kind()
			fvk := rt.Elem().Kind() //fvs.Type().Elem().Kind()
			val := reflect.MakeMap(rt)
			for k9, v9 := range mymap {
				k9c, v9c := ormCoerce(reflect.ValueOf(k9), fkk), ormCoerce(reflect.ValueOf(v9), fvk)
				val.SetMapIndex(k9c, v9c)
			}
			fr.Set(val)
		}
	case reflect.Ptr:
		rt2 = rt.Elem()
		if fr.IsNil() {
			fr.Set(reflect.New(rt2))
		}
		fr2 = fr.Elem()
		fallthrough
	case reflect.Struct:
		tm2, err := GetStructMetaFromType(rt2)
		if err != nil {
			return err
		}
		for j, _ := range tm2.DbFields {
			if fst, ok := rt2.FieldByName(j); ok {
				f2 := fr2.FieldByName(j)
				if fm2, ok := tm2.DbFields[fst.Name]; ok {
					mynm := fm.DbName + "_" + fm2.DbName
					ormGetSetVal(m, mynm, f2)
					//if myval, err := ormGetVal(m, mynm, f2.Type()); err == nil {
					//	ormSetVal(f2, myval) //f2.Set(reflect.ValueOf(val))
					//}
				}
			}
		}
	case reflect.Slice:
		rt3 := rt.Elem() // elem of the slice
		rt4 := rt3
		isPtrSlice := rt4.Kind() == reflect.Ptr
		if isPtrSlice {
			rt4 = rt4.Elem()
		}
		tm4, err := GetStructMetaFromType(rt4)
		if err != nil {
			return err
		}
		sval := reflect.MakeSlice(rt, 0, 4)
		//load up individual slices of all the keys
		allNmSlc := make(map[string]reflect.Value)
		allNmFM := make(map[string]*DbFieldMeta)
		allNms := make([]string, 0, 4)
		rlen := 0
		for _, mydbf := range tm4.DbFields {
			mynm0 := fm.DbName + "_" + mydbf.DbName
			logging.Trace(nil, "mydbf.ReflectType: %v, mynm0: %v", mydbf.ReflectType, mynm0)
			nmslc := reflect.MakeSlice(ormSlices[mydbf.ReflectType], 0, 4)
			if nmslc, err := ormGetSetVal(m, mynm0, nmslc); err == nil && nmslc.Len() > 0 {
				allNms = append(allNms, mynm0)
				if rlen == 0 {
					rlen = nmslc.Len()
				}
				allNmSlc[mynm0] = nmslc
				allNmFM[mynm0] = mydbf
			}
		}
		logging.Trace(nil, "rlen: %v, allNms: %v, allNmSlc: %v, allNmFM: %v",
			rlen, allNms, allNmSlc, allNmFM)
		//iterate through the allNmFM, and for each
		for j := 0; j < rlen; j++ {
			m0 := reflect.New(rt4)
			m1 := m0.Elem()
			for _, mynm := range allNms {
				f1 := m1.FieldByName(allNmFM[mynm].FieldName)
				ormSetVal(f1, allNmSlc[mynm].Index(j))
			}
			if isPtrSlice {
				m1 = m0
			}
			sval = reflect.Append(sval, m1)
		}
		fr.Set(sval)
		logging.Trace(nil, "slice sval: %#v", sval)
		//debug.PrintStack()
	}
	logging.Trace(nil, "returning from function")
	return nil
}

func ormFromIntfStruc(m *PropertyList, indexesOnly bool, fr reflect.Value,
	fm *DbFieldMeta, tm *TypeMeta) error {
	stf, _ := tm.Type.FieldByName(fm.FieldName) //fr.Type()
	rt := stf.Type
	rt2 := rt
	fr2 := fr
	logging.Trace(nil, " ***** tm.Type: %+v, field: %v, rt.Kind: %v", tm.Type, fm.FieldName, rt.Kind())
	switch rt.Kind() {
	case reflect.Map:
		mkeys := fr.MapKeys()
		nmK := fm.DbName + "_k"
		nmV := fm.DbName + "_v"
		for j := 0; j < len(mkeys); j++ {
			addToPropList(m, nmK, mkeys[j].Interface(), indexesOnly, invalidRV, true, fm, tm)
			addToPropList(m, nmV, fr.MapIndex(mkeys[j]).Interface(), indexesOnly, invalidRV, true, fm, tm)
		}
	case reflect.Ptr:
		rt2 = rt.Elem()
		fr2 = fr.Elem()
		fallthrough
	case reflect.Struct:
		//get names of all fields, and set them all
		tm2, err := GetStructMetaFromType(rt2)
		if err != nil {
			return err
		}
		for j, _ := range tm2.DbFields {
			if fst, ok := rt2.FieldByName(j); ok {
				if fm2, ok := tm2.DbFields[fst.Name]; ok {
					mynm, myval := fm.DbName+"_"+fm2.DbName, fr2.FieldByName(j).Interface()
					addToPropList(m, mynm, myval, indexesOnly, invalidRV, false, fm, tm)
				}
			}
		}
	case reflect.Slice:
		rt3 := rt.Elem()
		rlen := fr.Len()
		rt4 := rt3
		rtPtrSlice := rt4.Kind() == reflect.Ptr
		if rtPtrSlice {
			rt4 = rt4.Elem()
		}
		tm4, err := GetStructMetaFromType(rt4)
		if err != nil {
			return err
		}
		for j, _ := range tm4.DbFields {
			if fst, ok := rt4.FieldByName(j); ok {
				if fm2, ok := tm4.DbFields[fst.Name]; ok {
					logging.Trace(nil, "fst.Type: %v, .Name: %v, slice: %v",
						fst.Type, fst.Name, ormSlices[fst.Type.Kind()])

					for k := 0; k < rlen; k++ {
						fsv0 := fr.Index(k)
						if rtPtrSlice {
							fsv0 = fsv0.Elem()
						}
						mynm := fm.DbName + "_" + fm2.DbName
						//myval := ormCoerce(fsv0.FieldByName(j), fst.Type.Kind()).Interface()
						myval := fsv0.FieldByName(j).Interface()
						addToPropList(m, mynm, myval, indexesOnly, invalidRV, true, fm, tm)
					}
				}
			}
		}
	}
	return nil
}

//used when converting to a struct (during load from datastore)
//(no need to check for real type, since the field will have it appropriately)
func ormCoerce(val reflect.Value, rk reflect.Kind) reflect.Value {
	rkt := val
	var rki interface{}
	// we need to support all the different kinds, so we accomodate custom types around primitives
	switch rk {
	case reflect.Int:
		rki = int(val.Int())
	case reflect.Int8:
		rki = int8(val.Int())
	case reflect.Int16:
		rki = int16(val.Int())
	case reflect.Int32:
		rki = int32(val.Int())
	case reflect.Int64:
		rki = int64(val.Int())
	case reflect.Uint:
		rki = uint(val.Uint())
	case reflect.Uint8:
		rki = uint8(val.Uint())
	case reflect.Uint16:
		rki = uint16(val.Uint())
	case reflect.Uint32:
		rki = uint32(val.Uint())
	case reflect.Uint64:
		rki = uint64(val.Uint())
	case reflect.Uintptr:
		rki = uintptr(val.Uint())
	case reflect.Float32:
		rki = float32(val.Float())
	case reflect.Float64:
		rki = float64(val.Float())
	case reflect.String:
		rki = val.String()
	case reflect.Bool:
		rki = val.Bool()
	}
	if rki != nil {
		rkt = reflect.ValueOf(rki)
	}
	//logging.Trace(nil, "TTTTTTTT ormCoerce: val: %+v, kind: %v, return: %v", val, rk, rkt)
	return rkt
}

func ormSetValFnp(x interface{}, xt reflect.Value) string {
	return fmt.Sprintf("set fail. value %v overflows value of type %v", x, xt.Type())
}

//only call this if setting a primitive type (e.g. int, float, etc)
//This is called during load from datastore
//(no need to check for real type, since the field will have it appropriately)
//panics if anything bad occurs (also check overflow)
func ormSetVal(fv reflect.Value, val reflect.Value) {
	switch fv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		myval := val.Int()
		if fv.OverflowInt(myval) {
			panic(ormSetValFnp(myval, fv))
		}
		fv.SetInt(val.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		myval := val.Uint()
		if fv.OverflowUint(myval) {
			panic(ormSetValFnp(myval, fv))
		}
		fv.SetUint(myval)
	case reflect.Float32, reflect.Float64:
		myval := val.Float()
		if fv.OverflowFloat(myval) {
			panic(ormSetValFnp(myval, fv))
		}
		fv.SetFloat(myval)
	case reflect.String:
		fv.SetString(val.String())
	case reflect.Bool:
		fv.SetBool(val.Bool())
	default:
		logging.Trace(nil, "Unknown entity type in ormSetValue: %v", val)
		fv.Set(val)
	}
}

func ormGetSetVal(m *PropertyList, key string, rV reflect.Value) (reflect.Value, error) {
	isSlice := false
	typ := rV.Type()
	elemTyp := typ
	//var slcVal reflect.Value
	if typ != byteArrTyp && typ.Kind() == reflect.Slice {
		isSlice = true
		//slcVal = reflect.MakeSlice(typ, 0, 4)
		elemTyp = typ.Elem()
		//case reflect.Ptr: elemTyp = typ.Elem()
	}
	logging.Trace(nil, "XXXXXXX: elemTyp: %v, %#v", elemTyp, elemTyp)
	for _, val := range *m {
		if val.Name == key {
			//convert it to expected type
			//return it or add to slice
			elemVal := rV
			if isSlice {
				elemVal = reflect.New(elemTyp).Elem()
			}
			logging.Trace(nil, "XXXXXXX: elemVal: %v, %#v", elemVal, elemVal)
			dVal := reflect.ValueOf(val.Value)
			logging.Trace(nil, "XXXXXXX: dVal: %v, %#v", dVal, dVal)
			ormSetVal(elemVal, dVal)
			if isSlice {
				rV = reflect.Append(rV, elemVal)
			} else {
				//rV.Set(elemVal)
				return rV, nil
			}
		}
	}
	if isSlice && rV.Len() > 0 {
		return rV, nil
	}
	return invalidRV, fmt.Errorf("No properties found with name: %v", key)
}

//This is called during save
//check for and support all types in datastore
//(especially *Key, BlobKey and Time)
func addToPropList(m *PropertyList, name string, val interface{}, indexesOnly bool,
	fv reflect.Value, isSlice bool, fm *DbFieldMeta, tm *TypeMeta) {
	logging.Trace(nil, "addToPropList: name: %v, val: %v, rV: %v, rkind: %v",
		name, val, fv, fv.Kind())
	if fv.IsValid() {
		switch fv.Kind() {
		case reflect.Slice, reflect.Array:
			rlen := fv.Len()
			for j := 0; j < rlen; j++ {
				addToPropList(m, name, fv.Index(j).Interface(), indexesOnly, invalidRV, true, fm, tm)
			}
			return
		}
	}

	logging.Trace(nil, "Attempt add to property list: name: %v, val: %v, %#v", name, val, val)
	//We can't check individually if we should store rows, since we have to store all or
	//nothing of combo objects (maps, slices, struct, etc).
	//stre := fvc('s', val, fm, tm)
	//if ! stre {
	//	logging.Trace(nil, "FVC Store: False. Skipping: key: %v, value: %v", name, val)
	//	return
	//}

	proceedAndSet := true
	//if false { debug.PrintStack() }
	//ensure number type is int64 or float64
	fv = reflect.ValueOf(val)
	//must reflect on all of them (in case we have overloaded types e.g. enums)
	switch fv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val = fv.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		val = int64(fv.Uint())
	case reflect.Float32, reflect.Float64:
		val = fv.Float()
	case reflect.String:
		//logging.Trace(nil, "String. Type: %v, cdBlobkeyTyp: %v, val: %v",
		//	fv.Type(), richapp.lobkeyTyp, fv.String())
		//sval := fv.String()
		//switch fv.Type() {
		//case richapp.lobkeyTyp:
		//	val = appengine.BlobKey(sval)
		//	if strings.TrimSpace(sval) == "" {
		//		proceedAndSet = false
		//	}
		//default:
		//	val = sval
		//}
		//FIXME: app engine barfs if blobkey is blank
		val = fv.String()
	case reflect.Bool:
		val = fv.Bool()
	}
	if proceedAndSet {
		logging.Trace(nil, "Adding to property list: name: %v, val: %v, %#v", name, val, val)
		indx := fvc('i', val, fm, tm)
		if !indexesOnly || indx {
			*m = append(*m, Property{Name: name, Value: val, NoIndex: !indx, Multiple: isSlice})
		}
		logging.Trace(nil, "addToPropList: PropertyList: %#v", *m)
	}
}

//Check if we should store/index this.
//It can take full slices (when checking if to store), and also
//take individual slice members (when checking if to index).
//what = 's' or 'i' (for store or index respectively)
func fvc(what int, val interface{}, fm *DbFieldMeta, tm *TypeMeta) (b bool) {
	var (
		fvcs   []FieldValueChecker
		rv     reflect.Value
		rvkind reflect.Kind
	)
	switch what {
	case 's':
		fvcs = fm.Store
	case 'i':
		//if index checking, say no to []byte, and string with length > 500
		if _, ok := val.([]byte); ok {
			return
		} else {
			rv = reflect.ValueOf(val)
			rvkind = rv.Kind()
			if rvkind == reflect.String && len(rv.String()) > 500 {
				return
			}
		}
		fvcs = fm.Index
	default:
		panic("only 's' or 'i' is supported in fvc")
	}
	fmElemType := fm.ReflectType
	if fmElemType.Kind() == reflect.Slice {
		fmElemType = fmElemType.Elem()
	}
	b = true
	for i := 0; i < len(fvcs); i++ {
		inv := false
		switch fvcs[i] {
		case NOT_ALWAYS_FVC:
			inv = true
			fallthrough
		case ALWAYS_FVC:
			b = true
		case NOT_EMPTY_FVC:
			inv = true
			fallthrough
		case EMPTY_FVC:
			//b = true if has len and == 0, or is equal to the zero value
			if !rv.IsValid() {
				rv = reflect.ValueOf(val)
				rvkind = rv.Kind()
			}
			switch rvkind {
			case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
				b = (rv.Len() == 0)
			case reflect.Struct:
				b = (val == reflect.Zero(rv.Type()).Interface())
			default:
				b = (val == reflect.Zero(fmElemType).Interface())
			}
		}
		if inv {
			b = !b
		}
		if !b {
			break
		}
	}
	return
}

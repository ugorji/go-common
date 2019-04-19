package util

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	invalidRV    = reflect.Value{}
	intfSliceTyp = reflect.TypeOf([]interface{}(nil))
	intfTyp      = intfSliceTyp.Elem()
)

type ValuePrintfer struct {
	V interface{}
}

func (v ValuePrintfer) String() string {
	if v.V == nil {
		return ""
	}
	return ValuePrintf(v.V)
}

//Coerce a value into one of a different type.
//Returns the coerced value.
//Delegates to CoerceRV. (Wrapper of CoerceRV, without need to import reflect)
//Example:
//    Coerce(int(0), bool(false)) ==> returns a bool
//    Coerce([]interface{}{"a", "b", "c"}, []string(nil)) ==> []string{"a", "b", "c"}
func Coerce(val interface{}, typ interface{}) (interface{}, error) {
	rtyp := reflect.TypeOf(typ)
	rval := reflect.ValueOf(val)
	rval2, err := CoerceRV(rval, rtyp)
	return rval2.Interface(), err
}

//Coerce a value into one of a different type.
//See table below for source and possible destinations.
//    SOURCE    POSSIBLE DESTINATIONS
//    ===================================
//  - intXXX:   floatXXX, intXXX, uintXXX
//  - floatXXX: floatXXX, intXXX, uintXXX
//  - uintXXX:  floatXXX, intXXX, uintXXX
//  - string:   ANY (using fmt.Sprintf)
//  - bool:     ANY (using strconv.Atob)
//  - slice:    slice of any of above
//  - map:      mapping of any of above, or struct
//  - struct:   set fields based on field names of struct, or of passed map
func CoerceRV(rval reflect.Value, rtyp reflect.Type) (reflect.Value, error) {
	if rtyp.Kind() == reflect.Ptr {
		rtyp = rtyp.Elem()
	}
	rv0 := reflect.New(rtyp)
	rv := rv0.Elem()
	//if nil is passed, return the zero value
	//FIXME: if !rval.IsValid() || rval.IsNil() { return rv, nil }
	if !rval.IsValid() {
		return invalidRV, nil
	}
	coerMsg := "Kind: %v not coercible from val: %v of kind: %v"
	coerceNotDone := false
	if rval.Kind() == reflect.Interface {
		rval = reflect.ValueOf(rval.Interface())
	}
	switch rtyp.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		//supports float, int, uint
		xv := int64(0)
		xval := &xv
		switch rval.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			*xval = int64(rval.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			*xval = int64(rval.Uint())
		case reflect.Float32, reflect.Float64:
			*xval = int64(rval.Float())
		default:
			coerceNotDone = true
		}
		if coerceNotDone {
			return invalidRV, fmt.Errorf(coerMsg, rtyp.Kind(), rval.Interface(), rval.Kind())
		}
		rv.SetInt(*xval)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		//supports float, int, uint
		xv := uint64(0)
		xval := &xv
		switch rval.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			*xval = uint64(rval.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			*xval = uint64(rval.Uint())
		case reflect.Float32, reflect.Float64:
			*xval = uint64(rval.Float())
		default:
			coerceNotDone = true
		}
		if coerceNotDone {
			return invalidRV, fmt.Errorf(coerMsg, rtyp.Kind(), rval.Interface(), rval.Kind())
		}
		rv.SetUint(*xval)
		//rv.SetUint(uint64(rval.Float())) //json uses float
	case reflect.Float32, reflect.Float64:
		//supports float, int, uint
		xv := float64(0)
		xval := &xv
		switch rval.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			*xval = float64(rval.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			*xval = float64(rval.Uint())
		case reflect.Float32, reflect.Float64:
			*xval = float64(rval.Float())
		default:
			coerceNotDone = true
		}
		if coerceNotDone {
			return invalidRV, fmt.Errorf(coerMsg, rtyp.Kind(), rval.Interface(), rval.Kind())
		}
		rv.SetFloat(*xval)
		//rv.SetFloat(rval.Float())
	case reflect.String:
		xv := ""
		xval := &xv
		switch rval.Kind() {
		case reflect.String:
			*xval = rval.String()
		default:
			*xval = fmt.Sprintf("%v", rval.Interface())
		}
		if coerceNotDone {
			return invalidRV, fmt.Errorf(coerMsg, rtyp.Kind(), rval.Interface(), rval.Kind())
		}
		rv.SetString(*xval)
	case reflect.Bool:
		//supports bool, and anything else
		xv := false
		xval := &xv
		switch rval.Kind() {
		case reflect.Bool:
			*xval = rval.Bool()
		default:
			*xval, _ = strconv.ParseBool(fmt.Sprintf("%v", rval.Interface()))
		}
		if xval == nil {
			return invalidRV, fmt.Errorf(coerMsg, rtyp.Kind(), rval.Interface(), rval.Kind())
		}
		rv.SetBool(*xval)
	case reflect.Slice:
		type2 := rtyp.Elem()
		slen := rval.Len()
		rv = reflect.MakeSlice(rtyp, slen, slen)
		for i := 0; i < slen; i++ {
			rv2, err2 := CoerceRV(rval.Index(i), type2)
			if err2 != nil {
				return invalidRV, err2
			}
			rv.Index(i).Set(rv2)
			//rv = reflect.Append(rv, rv2)
		}
	case reflect.Map:
		rv := reflect.MakeMap(rtyp)
		mkeytyp := rtyp.Key()
		mvaltyp := rtyp.Elem()
		for _, mkey := range rval.MapKeys() {
			var mval reflect.Value
			rvkind := rval.Type().Kind()
			switch rvkind {
			case reflect.Struct:
				mval = rval.FieldByName(fmt.Sprintf("%v", mkey))
			case reflect.Map:
				mval = rval.MapIndex(mkey)
			default:
				return invalidRV, fmt.Errorf("Expect struct or map. Got: %v", rvkind)
			}
			rv1, err2 := CoerceRV(mkey, mkeytyp)
			if err2 != nil {
				return invalidRV, err2
			}
			rv2, err2 := CoerceRV(mval, mvaltyp)
			if err2 != nil {
				return invalidRV, err2
			}
			rv.SetMapIndex(rv1, rv2)
		}
	case reflect.Struct:
		//iterate through all public fields, and set them
		for fname, sf := range AllFieldNames(rtyp, true, nil) {
			var mval reflect.Value
			rvkind := rval.Type().Kind()
			switch rvkind {
			case reflect.Struct:
				mval = rval.FieldByName(fname)
			case reflect.Map:
				mval = rval.MapIndex(reflect.ValueOf(fname))
			default:
				return invalidRV, fmt.Errorf("Expect struct or map. Got: %v", rvkind)
			}
			rv1, err2 := CoerceRV(mval, sf.Type)
			if err2 != nil {
				return invalidRV, err2
			}
			rv.FieldByName(fname).Set(rv1)
		}
	default:
		return invalidRV, fmt.Errorf("Unsupported type: %v", rtyp)
	}
	logfn("CoerceRV: from: %v (%v), into type: %v, gives: %v (%#v)",
		rval, rval.Interface(), rtyp, rv, rv.Interface())
	return rv, nil
}

//Return all the field names, mapped to their struct fields.
//It does this recursively, getting for anonymous fields also.
func AllFieldNames(rt reflect.Type, exported bool, names map[string]reflect.StructField,
) map[string]reflect.StructField {
	if names == nil {
		names = make(map[string]reflect.StructField)
	}
	doAdd := true
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		if sf.Anonymous {
			names = AllFieldNames(sf.Type, exported, names)
		} else {
			if _, ok := names[sf.Name]; !ok {
				if exported {
					rune0, _ := utf8.DecodeRuneInString(sf.Name)
					doAdd = unicode.IsUpper(rune0)
				}
				if doAdd {
					names[sf.Name] = sf
				}
			}
		}
	}
	return names
}

var timeTyp = reflect.TypeOf(time.Time{})

type iszeroIntf interface {
	IsZero() bool
}

var iszeroTyp = reflect.TypeOf((*iszeroIntf)(nil)).Elem()

func IsEmptyValue(v reflect.Value, deref, checkStruct bool) bool {
	if !v.IsValid() {
		return true
	}
	vt := v.Type()
	if vt.Implements(iszeroTyp) {
		return v.Interface().(iszeroIntf).IsZero()
	}
	switch v.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		if deref {
			if v.IsNil() {
				return true
			}
			return IsEmptyValue(v.Elem(), deref, checkStruct)
		}
		return v.IsNil()
	case reflect.Struct:
		// check for time.Time, and return true if IsZero
		if vt == timeTyp {
			return v.Interface().(time.Time).IsZero()
		}
		if !checkStruct {
			return false
		}
		if vt.Comparable() {
			return v.Interface() == reflect.Zero(vt).Interface()
		}
		// return true if all fields are empty. else return false.
		// we cannot use equality check, because some fields may be maps/slices/etc
		// and consequently the structs are not comparable.
		// return v.Interface() == reflect.Zero(v.Type()).Interface()
		for i, n := 0, v.NumField(); i < n; i++ {
			if !IsEmptyValue(v.Field(i), deref, checkStruct) {
				return false
			}
		}
		return true
	}
	return false
}

// IndirIntf will take an interface and loop indirections on it till you get the real value.
// It is more critical for nil interfaces, as we can easily get lost in the indirection hell.
func Indir(rv reflect.Value, finalTyp reflect.Type, maxDepth int) reflect.Value {
	//fmt.Printf("111111: finalTyp: %v, intfTyp: %v, ==: %v\n", finalTyp, intfTyp, finalTyp == intfTyp)
	if !rv.IsValid() {
		return rv
	}
	//treat intfType as nil (and just flatten all the way)
	if finalTyp == intfTyp {
		finalTyp = nil
	}
	//fmt.Printf("$$$$$$$$$$$$$$$$$$: rk: %v, rv.CanAddr(): %v, rv: %v\n", rv.Kind(), rv.CanAddr(), rv)
	if maxDepth <= 0 {
		maxDepth = math.MaxInt16
	}
	for i := 0; i < maxDepth; i++ {
		if finalTyp != nil && rv.Type() == finalTyp {
			break
		}
		rk := rv.Kind()
		if !(rk == reflect.Ptr || rk == reflect.Interface) {
			break
		}
		rv2 := rv.Elem()
		if !rv2.IsValid() {
			break
		}
		rv = rv2
		//fmt.Printf("$$$$$$$$$$$$$$$$$$: rk: %v, rv.CanAddr(): %v, rv: %v\n", rk.String(), rv.CanAddr(), rv)
	}
	return rv
}

// fmt.Sprintf "%#v" or "%v" doesn't give information on contents of pointers
// (just shows pointer value).
// This allows us show a map, array, slice or struct containing pointers,
// while seeing their true value.
func ValuePrintf(v interface{}) string {
	buf := new(bytes.Buffer)
	if v2, ok := v.(reflect.Value); ok {
		valuePrintfRV(buf, v2, "", nil)
	} else {
		valuePrintfRV(buf, reflect.ValueOf(v), "", nil)
	}
	return buf.String()
}

func valuePrintfRV(buf *bytes.Buffer, rv reflect.Value, ptraddr string, ptrlist []interface{}) {
	// println("+")
	// with pointers, its possible to get into an infinite loop if some structs keep references
	// to themselves. To prevent this, we keep a list of pointers we've seen.
	// If we see a pointer a second time, we just print its ptr value, else we print
	// the full contents.
	switch rv.Kind() {
	case reflect.Ptr:
		if rv.IsNil() {
			buf.WriteString("<nil>")
		} else if rv.CanInterface() {
			pti := rv.Interface()
			isInPtrList := false
			for _, pt := range ptrlist {
				if pt == pti {
					isInPtrList = true
					break
				}
			}
			ptraddr = fmt.Sprintf("%p", pti)
			if isInPtrList {
				buf.WriteString(ptraddr)
			} else {
				ptrlist = append(ptrlist, pti)
				valuePrintfRV(buf, rv.Elem(), ptraddr, ptrlist)
			}
		} else {
			buf.WriteString("<*???>")
		}
	case reflect.Interface:
		valuePrintfRV(buf, rv.Elem(), ptraddr, ptrlist)
	case reflect.Slice, reflect.Array:
		buf.WriteString("[")
		rvlen := rv.Len()
		for i := 0; i < rvlen; i++ {
			if i != 0 {
				buf.WriteString(", ")
			}
			valuePrintfRV(buf, rv.Index(i), ptraddr, ptrlist)
		}
		buf.WriteString("]")
	case reflect.Map:
		buf.WriteString("{")
		mkeys := rv.MapKeys()
		for i := 0; i < len(mkeys); i++ {
			if i != 0 {
				buf.WriteString(", ")
			}
			valuePrintfRV(buf, mkeys[i], ptraddr, ptrlist)
			buf.WriteString(": ")
			valuePrintfRV(buf, rv.MapIndex(mkeys[i]), ptraddr, ptrlist)
		}
		buf.WriteString("}")
	case reflect.Struct:
		//fmt.Fprintf(buf, "%+v", rv.Interface())
		rt := rv.Type()
		numfield := rv.NumField()
		pkgpath := rt.PkgPath()
		if ptraddr != "" {
			buf.WriteString("&")
		}
		if pkgpath != "" {
			if lslash := strings.LastIndex(pkgpath, "/"); lslash != -1 {
				pkgpath = pkgpath[lslash+1:]
			}
			buf.WriteString(pkgpath)
			buf.WriteString(".")
		}
		buf.WriteString(rt.Name())
		if ptraddr != "" {
			buf.WriteString("(")
			buf.WriteString(ptraddr)
			buf.WriteString(")")
		}
		buf.WriteString("{")
		firstDone := false
		// unexportedNotDone := true //only record unexported once
		for i := 0; i < numfield; i++ {
			fname := rt.Field(i).Name
			// //unfortunately, cannot call rv.Interface() on unexported fields.
			// //and don't want to go thru iterating every value type.
			// //if we see unexported types, just put a ... in the output as a notifier.
			// printUnexported := false
			// if fname[0] < 'A' || fname[0] > 'Z' {
			// 	if unexportedNotDone {
			// 		printUnexported = true
			// 		unexportedNotDone = false
			// 	} else {
			// 		continue
			// 	}
			// }
			// if firstDone {
			// 	buf.WriteString(", ")
			// } else {
			// 	firstDone = true
			// }
			// if printUnexported {
			// 	buf.WriteString("...")
			// } else {
			// 	buf.WriteString(fname)
			// 	buf.WriteString(": ")
			// 	valuePrintfRV(buf, rv.Field(i), ptraddr, ptrlist)
			// }
			// Always print all fields, since we can check below if its okay.
			if firstDone {
				buf.WriteString(", ")
			} else {
				firstDone = true
			}
			buf.WriteString(fname)
			buf.WriteString(": ")
			valuePrintfRV(buf, rv.Field(i), ptraddr, ptrlist)
		}
		buf.WriteString("}")
	case reflect.String:
		fmt.Fprintf(buf, "%q", rv.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fmt.Fprintf(buf, "%v", rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		fmt.Fprintf(buf, "%v", rv.Uint())
	case reflect.Bool:
		fmt.Fprintf(buf, "%v", rv.Bool())
	case reflect.Float32, reflect.Float64:
		fmt.Fprintf(buf, "%v", rv.Float())
	case reflect.Complex64, reflect.Complex128:
		fmt.Fprintf(buf, "%v", rv.Complex())
	case reflect.Chan:
		fmt.Fprintf(buf, "<C>%p", rv.Pointer())
	case reflect.Func:
		fmt.Fprintf(buf, "<F>%p", rv.Pointer())
	case reflect.UnsafePointer:
		fmt.Fprintf(buf, "<U>%p", rv.Pointer())
	case reflect.Invalid:
		buf.WriteString("<nil>")
	default:
		buf.WriteString("<unknown>")
		// buf.WriteString(fmt.Sprintf("%v", rv.Interface()))
	}
}

// IndirIntf will take an interface and loop indirections on it till you get the real value.
func IndirIntf(v interface{}, finalTyp reflect.Type, maxDepth int) interface{} {
	return Indir(reflect.ValueOf(v), finalTyp, maxDepth).Interface()
}

// ToGeneric takes an interface{} and returns a Value interface{}
// with all references to custom types removed.
// The returned value contains only built-in data primitives
// (intX, uintX, floatX, bool, String, Slice, Map).
func ToGeneric(in interface{}) (out interface{}) {
	rv := reflect.ValueOf(in)
	rvi := Indir(rv, nil, -1)
	out = rvi.Interface()
	rti := rvi.Type()
	switch rvi.Kind() {
	case reflect.Struct:
		m := make(map[string]interface{})
		out = m
		for i := 0; i < rvi.NumField(); i++ {
			sf := rti.Field(i)
			fv := rvi.Field(i)
			switch sf.Type.Kind() {
			case reflect.Chan, reflect.Func, reflect.Ptr,
				reflect.Interface, reflect.Invalid, reflect.UnsafePointer:
				//do nothing (for all skippable types)
			default:
				m[sf.Name] = ToGeneric(fv.Interface())
			}
		}
	case reflect.Slice, reflect.Array:
		//return new slice with all types generic
		//look at type/kind of slice, and if not generic, create new one and convert each
		l := make([]interface{}, rvi.Len())
		for i := 0; i < rvi.Len(); i++ {
			l[i] = ToGeneric(rvi.Index(i).Interface())
		}
		out = l
	case reflect.Map:
		//look at key/value and make generic replacement
		m := make(map[interface{}]interface{})
		out = m
		for _, mk := range rvi.MapKeys() {
			mk0 := ToGeneric(mk)
			if mk0 != nil {
				m[mk0] = ToGeneric(rvi.MapIndex(mk))
			}
		}
	case reflect.Chan, reflect.Func, reflect.Ptr,
		reflect.Interface, reflect.Invalid, reflect.UnsafePointer:
		out = nil
		//do nothing (for all skippable types)
	default:
		//use already set out
	}
	return
}

func ExpandSliceValue(s reflect.Value, num int) reflect.Value {
	if num <= 0 {
		return s
	}
	l0 := s.Len()
	l1 := l0 + num // new slice length
	if l1 < l0 {
		panic("ExpandSlice: slice overflow")
	}
	c0 := s.Cap()
	if l1 <= c0 {
		return s.Slice(0, l1)
	}
	st := s.Type()
	c1 := GrowCap(c0, int(st.Elem().Size()), num)
	s2 := reflect.MakeSlice(st, l1, c1)
	// println("expandslicevalue: cap-old: ", c0, ", cap-new: ", c1, ", len-new: ", l1)
	reflect.Copy(s2, s)
	return s2
}

func ApproxDataSize(rv reflect.Value) (sum int) {
	return approxDataSize(rv)
}

func approxDataSize(rv reflect.Value) (sum int) {
	switch rk := rv.Kind(); rk {
	case reflect.Invalid:
	case reflect.Ptr, reflect.Interface:
		sum += int(rv.Type().Size())
		sum += approxDataSize(rv.Elem())
	case reflect.Slice:
		sum += int(rv.Type().Size())
		for j := 0; j < rv.Len(); j++ {
			sum += approxDataSize(rv.Index(j))
		}
	case reflect.String:
		sum += int(rv.Type().Size())
		sum += rv.Len()
	case reflect.Map:
		sum += int(rv.Type().Size())
		for _, mk := range rv.MapKeys() {
			sum += approxDataSize(mk)
			sum += approxDataSize(rv.MapIndex(mk))
		}
	case reflect.Struct:
		//struct size already includes the full data size.
		//sum += int(rv.Type().Size())
		for j := 0; j < rv.NumField(); j++ {
			sum += approxDataSize(rv.Field(j))
		}
	default:
		//pure value types
		sum += int(rv.Type().Size())
	}
	return
}

type deepEqualOpts struct {
	typeMustBeEqual           bool
	containerNilEqualsZeroLen bool
}

// This checks 2 interfaces to see if they are the same.
// If strict=true, then checks that they are also same type (while walking the interfaces).
// It skips functions, channels, and non-exported fields in structs.
// It is better than the DeepEqual in reflect because it gives contextual
// information back in the error on what was wrong.
func DeepEqual(v1, v2 interface{}, strict bool) (err error) {
	var v1r, v2r reflect.Value
	var ok bool
	if v1r, ok = v1.(reflect.Value); !ok {
		v1r = reflect.ValueOf(v1)
	}
	if v2r, ok = v2.(reflect.Value); !ok {
		v2r = reflect.ValueOf(v2)
	}
	if strict {
		return deepValueEqual(v1r, v2r, "", deepEqualOpts{true, false})
	}
	return deepValueEqual(v1r, v2r, "", deepEqualOpts{false, true})
}

func deepValueEqual(v1, v2 reflect.Value, ctx string, t deepEqualOpts) (err error) {
	if v1.IsValid() && !v2.IsValid() {
		err = fmt.Errorf("comparing valid to non-valid value: %s (%v)", ctx, v1)
		return
	}
	if !v1.IsValid() && v2.IsValid() {
		err = fmt.Errorf("comparing non-valid to valid value: %s (%v)", ctx, v2)
		return
	}
	if !v1.IsValid() && !v2.IsValid() {
		return
	}
	// we know they are both valid

	if v1.CanAddr() && v2.CanAddr() {
		if v1.UnsafeAddr() == v2.UnsafeAddr() {
			return
		}
	}

CHECK1:
	switch v1.Kind() {
	case reflect.Ptr:
		if v1.IsNil() {
			if !(v2.Kind() == reflect.Ptr && v2.IsNil()) {
				err = fmt.Errorf("compare nil to non-nil pointer: %s (%v)", ctx, v2)
			}
			return
		}
		v1 = v1.Elem()
		goto CHECK1
	case reflect.Interface:
		if v1.IsNil() {
			if !(v2.Kind() == reflect.Interface && v2.IsNil()) {
				err = fmt.Errorf("compare nil to non-nil interface: %s (%v)", ctx, v2)
			}
			return
		}
		v1 = v1.Elem()
		goto CHECK1
	case reflect.Slice:
		if v1.IsNil() {
			if v2.Kind() == reflect.Slice {
				if !v2.IsNil() {
					err = fmt.Errorf("comparing nil to non-nil slice: %s (%v)", ctx, v2)
				} else if t.containerNilEqualsZeroLen && v2.Len() != 0 {
					err = fmt.Errorf("comparing nil to non zero-length slice: %s (%v)", ctx, v2)
				}
			} else {
				err = fmt.Errorf("comparing nil slice to non-slice: %s (%v)", ctx, v2)
			}
			return
		}
	case reflect.Map:
		if v1.IsNil() {
			if v2.Kind() == reflect.Map {
				if !v2.IsNil() {
					err = fmt.Errorf("comparing nil to non-nil map: %s (%v)", ctx, v2)
				} else if t.containerNilEqualsZeroLen && v2.Len() != 0 {
					err = fmt.Errorf("comparing nil to non zero-length map: %s (%v)", ctx, v2)
				}
			} else {
				err = fmt.Errorf("comparing nil map to non-map: %s (%v)", ctx, v2)
			}
			return
		}
	case reflect.Chan:
		if v1.IsNil() {
			if !(v2.Kind() == reflect.Chan && v2.IsNil()) {
				err = fmt.Errorf("compare nil to non-nil chan: %s (%v)", ctx, v2)
			}
			return
		}
		err = fmt.Errorf("cannot compare chan types: %s", ctx)
		return
	case reflect.Func:
		if v1.IsNil() {
			if !(v2.Kind() == reflect.Func && v2.IsNil()) {
				err = fmt.Errorf("compare nil to non-nil func: %s (%v)", ctx, v2)
			}
			return
		}
		err = fmt.Errorf("cannot compare func types: %s", ctx)
		return
	}

	// now v1 is non-nil, and not an array, ptr, interface, chan, or func (or other nil-able type)

	v3 := v1

CHECK2:
	switch v2.Kind() {
	case reflect.Ptr:
		if v2.IsNil() {
			if !(v3.Kind() == reflect.Ptr && v3.IsNil()) {
				err = fmt.Errorf("compare non-nil to nil pointer: %s (%v)", ctx, v3)
			}
			return
		}
		v2 = v2.Elem()
		goto CHECK2
	case reflect.Interface:
		if v2.IsNil() {
			if !(v3.Kind() == reflect.Interface && v3.IsNil()) {
				err = fmt.Errorf("compare non-nil to nil interface: %s (%v)", ctx, v3)
			}
			return
		}
		v2 = v2.Elem()
		goto CHECK2
	case reflect.Slice:
		if v2.IsNil() {
			if v3.Kind() == reflect.Slice {
				if !v3.IsNil() {
					err = fmt.Errorf("comparing non-nil to nil slice: %s (%v)", ctx, v3)
				} else if t.containerNilEqualsZeroLen && v3.Len() != 0 {
					err = fmt.Errorf("comparing non zero-length to nil slice: %s (%v)", ctx, v3)
				}
			} else {
				err = fmt.Errorf("comparing non-slice to nil slice: %s (%v)", ctx, v3)
			}
			return
		}
	case reflect.Map:
		if v2.IsNil() {
			if v3.Kind() == reflect.Map {
				if !v3.IsNil() {
					err = fmt.Errorf("comparing non-nil to nil map: %s (%v)", ctx, v3)
				} else if t.containerNilEqualsZeroLen && v3.Len() != 0 {
					err = fmt.Errorf("comparing non zero-length to nil map: %s (%v)", ctx, v3)
				}
			} else {
				err = fmt.Errorf("comparing non-map to nil map: %s (%v)", ctx, v3)
			}
			return
		}
	case reflect.Chan:
		if v2.IsNil() {
			if !(v3.Kind() == reflect.Chan && v3.IsNil()) {
				err = fmt.Errorf("compare nil to non-nil chan: %s (%v)", ctx, v3)
			}
			return
		}
		err = fmt.Errorf("cannot compare chan types: %s", ctx)
		return
	case reflect.Func:
		if v2.IsNil() {
			if !(v3.Kind() == reflect.Func && v3.IsNil()) {
				err = fmt.Errorf("compare nil to non-nil func: %s (%v)", ctx, v3)
			}
			return
		}
		err = fmt.Errorf("cannot compare func types: %s", ctx)
		return
	}

	// now v2 is non-nil, and not an array, ptr, interface, chan, or func (or other nil-able type)

	// now call the underlying code that just checks the values.

	if t.typeMustBeEqual && v1.Type() != v2.Type() {
		err = fmt.Errorf("types mismatch: %s (%v, %v)", ctx, v1.Type(), v2.Type())
		return
	}

	v1Kind, v2Kind := v1.Kind(), v2.Kind()
	if v1.Kind() != v2.Kind() &&
		!((v1Kind == reflect.Array && v2Kind == reflect.Slice) || (v1Kind == reflect.Slice && v2Kind == reflect.Array)) {
		// treat array and slice as similar kinds
		err = fmt.Errorf("kind mismatch: %s (%v, %v)", ctx, v1.Kind(), v2.Kind())
		return
	}

	// kinds are equal now.

	switch v1.Kind() {
	case reflect.Struct:
		for i, n := 0, v1.NumField(); i < n; i++ {
			if err = deepValueEqual(v1.Field(i), v2.Field(i), ctx+"/field:"+strconv.Itoa(i), t); err != nil {
				return
			}
		}
		return
	case reflect.Slice, reflect.Array:
		for i := 0; i < v1.Len(); i++ {
			if err = deepValueEqual(v1.Index(i), v2.Index(i), ctx+"/index:"+strconv.Itoa(i), t); err != nil {
				return
			}
		}
		return
	case reflect.Map:
		for _, k := range v1.MapKeys() {
			if err = deepValueEqual(v1.MapIndex(k), v2.MapIndex(k), fmt.Sprintf("%s/key:%v", ctx, k.Interface()), t); err != nil {
				return
			}
		}
		return
	case reflect.String:
		if v1.String() != v2.String() {
			err = fmt.Errorf("string mismatch: %s (%v, %v)", ctx, v1.String(), v2.String())
		}
		return
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v1.Int() != v2.Int() {
			err = fmt.Errorf("int mismatch: %s (%v, %v)", ctx, v1.Int(), v2.Int())
		}
		return
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if v1.Uint() != v2.Uint() {
			err = fmt.Errorf("uint mismatch: %s (%v, %v)", ctx, v1.Uint(), v2.Uint())
		}
		return
	case reflect.Bool:
		if v1.Bool() != v2.Bool() {
			err = fmt.Errorf("bool mismatch: %s (%v, %v)", ctx, v1.Bool(), v2.Bool())
		}
		return
	case reflect.Float32, reflect.Float64:
		if v1.Float() != v2.Float() {
			err = fmt.Errorf("float mismatch: %s (%v, %v)", ctx, v1.Float(), v2.Float())
		}
		return
	case reflect.Complex64, reflect.Complex128:
		if v1.Complex() != v2.Complex() {
			err = fmt.Errorf("complex number mismatch: %s (%v, %v)", ctx, v1.Complex(), v2.Complex())
		}
		return
	case reflect.UnsafePointer:
		if v1.Pointer() != v2.Pointer() {
			err = fmt.Errorf("unsafe pointer mismatch: %s (%v, %v)", ctx, v1.Pointer(), v2.Pointer())
		}
		return
	default:
		if v1.CanInterface() && v2.CanInterface() {
			if v1.Interface() != v2.Interface() {
				err = fmt.Errorf("interface{} mismatch: %s (%v, %v)", ctx, v1.Interface(), v2.Interface())
			}
		} else {
			err = fmt.Errorf("cannot compare types: %s (%v, %v)", ctx, v1, v2)
		}
		return
	}
	return
}

func ImplementsInterface(typ, iTyp reflect.Type) (success bool, indir int8) {
	if typ == nil {
		return
	}
	rt := typ
	// The type might be a pointer and we need to keep
	// dereferencing to the base type until we find an implementation.
	for {
		if rt.Implements(iTyp) {
			return true, indir
		}
		if p := rt; p.Kind() == reflect.Ptr {
			indir++
			if indir >= math.MaxInt8 { // insane number of indirections
				return false, 0
			}
			rt = p.Elem()
			continue
		}
		break
	}
	// No luck yet, but if this is a base type (non-pointer), the pointer might satisfy.
	if typ.Kind() != reflect.Ptr {
		// Not a pointer, but does the pointer work?
		if reflect.PtrTo(typ).Implements(iTyp) {
			return true, -1
		}
	}
	return false, 0
}

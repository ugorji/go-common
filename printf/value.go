package printf

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
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
	// If we see a pointer a second time, we just printf.its ptr value, else we print
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
			// printf.nexported := false
			// if fname[0] < 'A' || fname[0] > 'Z' {
			// 	if unexportedNotDone {
			// 		printf.nexported = true
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
			// if printf.nexported {
			// 	buf.WriteString("...")
			// } else {
			// 	buf.WriteString(fname)
			// 	buf.WriteString(": ")
			// 	valuePrintfRV(buf, rv.Field(i), ptraddr, ptrlist)
			// }
			// Always printf.all fields, since we can check below if its okay.
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

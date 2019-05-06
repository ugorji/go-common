// +build !safe
// +build !appengine

package runtimeutil

import "unsafe"

const safeMode = false

type unsafeString struct {
	Data unsafe.Pointer
	Len  int
}

type unsafeSlice struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

func StringView(v []byte) string {
	if len(v) == 0 {
		return ""
	}
	bx := (*unsafeSlice)(unsafe.Pointer(&v))
	return *(*string)(unsafe.Pointer(&unsafeString{bx.Data, bx.Len}))
}

func BytesView(v string) []byte {
	if len(v) == 0 {
		return []byte{}
	}
	sx := (*unsafeString)(unsafe.Pointer(&v))
	return *(*[]byte)(unsafe.Pointer(&unsafeSlice{sx.Data, sx.Len, sx.Len}))
}

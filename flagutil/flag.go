package flagutil

import (
	"fmt"
	"regexp"
	"strconv"
)

type RegexpFlagValue regexp.Regexp

func (v *RegexpFlagValue) Set(s string) (err error) {
	vv, err := regexp.Compile(s)
	if err == nil {
		*v = (RegexpFlagValue)(*vv)
	}
	return
}

func (v *RegexpFlagValue) Get() interface{} { return (*regexp.Regexp)(v) }

func (v *RegexpFlagValue) String() string { return fmt.Sprintf("%v", v.Get()) }

// boolFlagValue can be set to true or false, or unset as nil (default)
type BoolFlagValue struct {
	v *bool
}

func (v *BoolFlagValue) Bool() *bool { return v.v }

func (v *BoolFlagValue) Set(s string) (err error) {
	b, err := strconv.ParseBool(s)
	if err == nil {
		v.v = &b
	}
	return
}

func (v *BoolFlagValue) Get() interface{} { return v.v }

func (v *BoolFlagValue) String() string { return fmt.Sprintf("%#v", v.v) }

type SetStringFlagValue struct {
	v map[string]struct{}
}

func (v *SetStringFlagValue) SetString() map[string]struct{} { return v.v }

func (v *SetStringFlagValue) Set(s string) (err error) {
	if v.v == nil {
		v.v = make(map[string]struct{})
	}
	v.v[s] = struct{}{}
	return
}

func (v *SetStringFlagValue) Get() interface{} { return v.v }

func (v *SetStringFlagValue) String() string { return fmt.Sprintf("%v", v.v) }

type StringsFlagValue []string

func (v *StringsFlagValue) Set(s string) (err error) {
	*v = append(*v, s)
	return
}
func (v *StringsFlagValue) Get() interface{} { return ([]string)(*v) }
func (v *StringsFlagValue) String() string   { return fmt.Sprintf("%v", v.Get()) }

type StringsNoDupFlagValue []string

func (v *StringsNoDupFlagValue) Set(s string) (err error) {
	for _, vs := range *v {
		if vs == s {
			return
		}
	}
	*v = append(*v, s)
	return
}
func (v *StringsNoDupFlagValue) Get() interface{} { return ([]string)(*v) }
func (v *StringsNoDupFlagValue) String() string   { return fmt.Sprintf("%v", v.Get()) }

// type RegexpFlagValue struct {
// 	v *regexp.Regexp
// }

// func (v *RegexpFlagValue) Regexp() *regexp.Regexp { return v.v }

// func (v *RegexpFlagValue) Set(s string) (err error) {
// 	v.v, err = regexp.Compile(s)
// 	return
// }

// func (v *RegexpFlagValue) Get() interface{} { return v.v }

// func (v *RegexpFlagValue) String() string { return fmt.Sprintf("%v", v.v) }

////

// type StringsFlagValue struct {
// 	v          []string
// 	Duplicates bool
// }

// func (v *StringsFlagValue) Strings() []string { return v.v }

// func (v *StringsFlagValue) Set(s string) (err error) {
// 	if !Duplicates {
// 		for _, v := range v.v {
// 			if v == s {
// 				return
// 			}
// 		}
// 	}
// 	v.v = append(v.v, s)
// 	return
// }

// func (v *StringsFlagValue) Get() interface{} { return v.v }

// func (v *StringsFlagValue) String() string { return fmt.Sprintf("%v", v.v) }

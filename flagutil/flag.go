package flagutil

import (
	"fmt"
	"regexp"
	"strconv"
)

type RegexpFlagValue struct {
	v *regexp.Regexp
}

func (v *RegexpFlagValue) Regexp() *regexp.Regexp { return v.v }

func (v *RegexpFlagValue) Set(s string) (err error) {
	v.v, err = regexp.Compile(s)
	return
}

func (v *RegexpFlagValue) Get() interface{} { return v.v }

func (v *RegexpFlagValue) String() string { return fmt.Sprintf("%v", v.v) }

// boolFlagValue can be set to true or false, or unset as nil (default)
type BoolFlagValue struct {
	v *bool
}

func (v *BoolFlagValue) Bool() *bool { return v.v }

func (v *BoolFlagValue) Set(s string) (err error) {
	// zz.Debugf("calling boolFlagValue.Set with %s\n", s)
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

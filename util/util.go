package util

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
)

const (
	InterpolatePrefix  string = "${"
	InterpolatePostfix        = "}"
)

var LogFn func(format string, params ...interface{})

func logfn(format string, params ...interface{}) {
	if LogFn != nil {
		LogFn(format, params...)
	}
}

//Returns a UUID, given the max length. If maxlen <=10, use 16 as max len.
func UUID(xlen int) (string, error) {
	if xlen <= 10 {
		xlen = 16
	}
	b := make([]byte, xlen)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0F) | 0x40
	b[8] = (b[8] &^ 0x40) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func UUID2() (uuid []byte, err error) {
	uuid = make([]byte, 16)
	if _, err = io.ReadFull(rand.Reader, uuid); err != nil {
		return
	}
	// Set version (4) and variant (2).
	var version byte = 4 << 4
	var variant byte = 2 << 4
	uuid[6] = version | (uuid[6] & 15)
	uuid[8] = variant | (uuid[8] & 15)
	return
}

// Given a string like "abc${key1}def${key2}ghi", it will take vars like
// { key1: "_123_", key2: "-456-" } and return the interpolated string
// ie abc_123_def-456-ghi
func Interpolate(s string, vars map[string]interface{}) string {
	re := regexp.MustCompile(`\${[^}]*}`) //(`\${.*}`)
	//(src string, repl func(string) string) string
	fn := func(s1 string) string {
		logfn("Found Match: %v", s1)
		s11 := s1[2 : len(s1)-1]
		if s12, ok := vars[s11]; ok {
			if s13, ok := s12.(string); ok {
				return s13
			}
			return fmt.Sprintf("%v", s12)
		}
		return s1
	}
	s2 := re.ReplaceAllStringFunc(s, fn)
	return s2
}

func isSep(x byte) bool {
	return x == '/' || x == '\\'
}

func PathMatchesStaticFile(path string) (isMatch bool) {
	plen := len(path)
	if plen < 2 {
		return
	}
	var kexts = [...]string{
		"gif", "png", "txt", "jpg", "ico", "css", "js", "json", "txt",
		"mp4", "mpeg",
	}
	var ext string
	for i := plen - 1; i >= 0 && !isSep(path[i]); i-- {
		if path[i] == '.' {
			ext = path[i+1:]
			break
		}
	}
	if ext == "" {
		return
	}
	ext = strings.ToLower(ext)
	//println("XXXXXXXXXXXXXXXXXXXXXX ext #####################", ext)
	for _, kext := range kexts {
		if ext == kext {
			isMatch = true
			return
		}
	}
	return
}

func FindFreeLocalPort() (port int, err error) {
	var ln net.Listener
	if ln, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
		return
	}
	lnaddr := ln.Addr()
	if addr, ok := lnaddr.(*net.TCPAddr); ok {
		port = addr.Port
	} else {
		err = fmt.Errorf("listenaddress is not a TCPAddr: Type: %T, %v", lnaddr, lnaddr)
	}
	ln.Close()
	return
}

// Parse a template like: abc/${key1}/def/${key2:.+a?}/ghi, and return the following:
//   - regexp: abc/(.*)/def/(+a?)/ghi         (Can be matched against with groups)
//   - clean:  abc/{key1}/def/{key2}/ghi      (can be substituted against)
//   - takeys: [key1, key2]                   (so we know what keys map to the numbered groups)
//   - err:    nil                            (in case there's any error)
func ParseRegexTemplate(s string) (re *regexp.Regexp, clean string, takeys []string, err error) {
	type t struct{ open, colon, closer int }
	ta := make([]*t, 0, 4)
	slen := len(s)
	for i := 0; i < slen; i++ {
		if s[i] == '$' && s[i+1] == '{' {
			_i := i
			t0 := &t{open: i, colon: -1, closer: -1}
		L:
			for ; i < slen; i++ {
				switch s[i] {
				case ':':
					t0.colon = i
				case '}':
					t0.closer = i
					break L
				}
			}
			logfn("parseRegexTemplate: ${ at: %v, } at: %v", _i, i)
			if t0.closer < 0 {
				err = fmt.Errorf("No closing brace } for opening at: %v", strconv.Itoa(t0.open))
				return
			}
			ta = append(ta, t0)
		}
	}
	var bclean bytes.Buffer
	var bre bytes.Buffer
	talen := len(ta)
	logfn("ta: %v", ta)

	startpos := 0
	for i := 0; i < talen; i++ {
		startpos = 0
		if i > 0 {
			startpos = ta[i-1].closer + 1
		}

		bclean.WriteString(s[startpos:ta[i].open])
		bclean.WriteString(InterpolatePrefix)
		endpos := ta[i].closer
		if ta[i].colon >= 0 {
			endpos = ta[i].colon
		}
		bclean.WriteString(s[ta[i].open+2 : endpos])
		bclean.WriteString(InterpolatePostfix)

		takeys = append(takeys, s[ta[i].open+2:endpos])

		bre.WriteString(s[startpos:ta[i].open])
		if ta[i].colon > 0 {
			bre.WriteString("(")
			bre.WriteString(s[ta[i].colon+1 : ta[i].closer])
			bre.WriteString(")")
		} else {
			bre.WriteString(`([^/]*)`)
		}
	}
	startpos = 0
	if talen > 0 {
		startpos = ta[talen-1].closer + 1
	}
	bclean.WriteString(s[startpos:])
	bre.WriteString(s[startpos:])

	re, err = regexp.Compile(bre.String())
	if err != nil {
		return
	}
	clean = bclean.String()

	logfn("parseRegexTemplate: bre: %v, re: %v, clean: %v, takeys: %v, err: %v",
		bre, re, clean, takeys, err)
	return
}

func PruneSignExt(v []byte, pos bool) (n int) {
	if len(v) < 2 {
	} else if pos && v[0] == 0 {
		for ; v[n] == 0 && n+1 < len(v) && (v[n+1]&(1<<7) == 0); n++ {
		}
	} else if !pos && v[0] == 0xff {
		for ; v[n] == 0xff && n+1 < len(v) && (v[n+1]&(1<<7) != 0); n++ {
		}
	}
	return
}

func PruneLeading(v []byte, pruneVal byte) (n int) {
	if len(v) < 2 {
		return
	}
	for ; v[n] == pruneVal && n+1 < len(v) && (v[n+1] == pruneVal); n++ {
	}
	return
}

// validate that this function is correct ...
// culled from OGRE (Object-Oriented Graphics Rendering Engine)
// function: halfToFloatI (http://stderr.org/doc/ogre-doc/api/OgreBitwise_8h-source.html)
func HalfFloatToFloatBits(yy uint16) (d uint32) {
	y := uint32(yy)
	s := (y >> 15) & 0x01
	e := (y >> 10) & 0x1f
	m := y & 0x03ff

	if e == 0 {
		if m == 0 { // plu or minus 0
			return s << 31
		}
		// Denormalized number -- renormalize it
		for (m & 0x0400) == 0 {
			m <<= 1
			e -= 1
		}
		e += 1
		const zz uint32 = 0x0400
		m &= ^zz
	} else if e == 31 {
		if m == 0 { // Inf
			return (s << 31) | 0x7f800000
		}
		return (s << 31) | 0x7f800000 | (m << 13) // NaN
	}
	e = e + (127 - 15)
	m = m << 13
	return (s << 31) | (e << 23) | m
}

// GrowCap will return a new capacity for a slice, given the following:
//   - oldCap: current capacity
//   - unit: in-memory size of an element
//   - num: number of elements to add
func GrowCap(oldCap, unit, num int) (newCap int) {
	// design this well so it can be easily inlined.
	// This means:
	//   - no constants, panic, switch, etc

	// appendslice logic (if cap < 1024, *2, else *1.25):
	//   leads to many copy calls, especially when copying bytes.
	//   bytes.Buffer model (2*cap + n): much better for bytes.
	// smarter way is to take the byte-size of the appended element(type) into account

	// maintain 3 thresholds:
	// t1: if cap <= t1, newcap = 2x
	// t2: if cap <= t2, newcap = 1.75x
	// t3: if cap <= t3, newcap = 1.5x
	//     else          newcap = 1.25x
	//
	// t1, t2, t3 >= 1024 always.
	// This means that, if unit size >= 16, then always do 2x or 1.25x (ie t1, t2, t3 are all same)
	//
	// With this, appending for bytes increase by:
	//    100% up to 4K
	//     75% up to 8K
	//     50% up to 16K
	//     25% beyond that

	// unit can be 0 e.g. for struct{}{}; handle that appropriately
	// handle if num < 0, cap=0, etc.
	var t1, t2, t3 int // thresholds
	if unit <= 1 {
		t1, t2, t3 = 4*1024, 8*1024, 16*1024
	} else if unit < 16 {
		t3 = 16 / unit * 1024
		t1 = t3 * 1 / 4
		t2 = t3 * 2 / 4
	} else {
		t1, t2, t3 = 1024, 1024, 1024
	}

	var x int // temporary variable

	// x is multiplier here.
	// x is either 5, 6, 7 or 8 (for incr of 25%, 50%, 75% or 100% respectively)
	if oldCap <= t1 { // [0,t1]
		x = 8
	} else if oldCap > t3 { // (t3,infinity]
		x = 5
	} else if oldCap <= t2 { // (t1,t2]
		x = 7
	} else { // (t2,t3]
		x = 6
	}
	newCap = x * oldCap / 4

	if num > 0 {
		newCap += num
	}

	// ensure newCap is a multiple of 64 (if it is > 64)
	// we want this inlined, so DONT use a const declaration.
	// TODO: After inl handles ODCLCONST, then use a const for 65
	// const growToCapDiv = 64 // return caps as a multiple of 8 64-bit words
	if newCap > 64 {
		if x = newCap % 64; x != 0 {
			x = newCap / 64
			newCap = 64 * (x + 1)
		}
	} else {
		if x = newCap % 16; x != 0 {
			x = newCap / 16
			newCap = 16 * (x + 1)
		}
	}
	return
}

/*
// Converts a string like "60s, 2m, 4h, 5d" into appropriate seconds.
// If a string has no designation (e.g. just 2), treat it like seconds.
// Example:
//    60s, 2m, 3, 4h, 5d ==> 60 + 2*60 + 3 + 4*60*60 + 5*24*60*60
//    1d ==> 1*24*60*60
//    500 ==> 500
func FineTimeSecs(s string) (t int64, err error) {
	var t0 int64
	for _, s0 := range strings.Split(s, ",") {
		t1, err := oneFineTimeSecs(strings.Trim(s0, ", \t"))
		t0 = t0 + t1
		if err != nil {
			return t0, err
		}
	}
	return t0, nil
}

func oneFineTimeSecs(s string) (t int64, err error) {
	slen := len(s)
	s1 := s[0 : slen-1]
	s2 := s[slen-1 : slen]
	t1, err := strconv.ParseInt(s1, 10, 64)
	if err != nil {
		return
	}
	switch s2 {
	case "s":
		t = t1
	case "m":
		t = t1 * 60
	case "h":
		t = t1 * 60 * 60
	case "d":
		t = t1 * 60 * 60 * 24
	default:
		t, err = oneFineTimeSecs(s + "s")
		if err != nil {
			return
		}
	}
	return
}

*/

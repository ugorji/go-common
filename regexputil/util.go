package regexputil

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	interpolatePrefix  string = "${"
	interpolatePostfix        = "}"
)

func isSep(x byte) bool {
	return x == '/' || x == '\\'
}

// Given a string like "abc${key1}def${key2}ghi", it will take vars like
// { key1: "_123_", key2: "-456-" } and return the interpolated string
// ie abc_123_def-456-ghi
func Interpolate(s string, vars map[string]interface{}) string {
	re := regexp.MustCompile(`\${[^}]*}`) //(`\${.*}`)
	//(src string, repl func(string) string) string
	fn := func(s1 string) string {
		// logfn("Found Match: %v", s1)
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
			// _i := i
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
			// logfn("parseRegexTemplate: ${ at: %v, } at: %v", _i, i)
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
	// logfn("ta: %v", ta)

	startpos := 0
	for i := 0; i < talen; i++ {
		startpos = 0
		if i > 0 {
			startpos = ta[i-1].closer + 1
		}

		bclean.WriteString(s[startpos:ta[i].open])
		bclean.WriteString(interpolatePrefix)
		endpos := ta[i].closer
		if ta[i].colon >= 0 {
			endpos = ta[i].colon
		}
		bclean.WriteString(s[ta[i].open+2 : endpos])
		bclean.WriteString(interpolatePostfix)

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

	// logfn("parseRegexTemplate: bre: %v, re: %v, clean: %v, takeys: %v, err: %v",
	// 	bre, re, clean, takeys, err)
	return
}

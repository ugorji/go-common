package logging

import (
	"math"

	"github.com/ugorji/go/codec"
)

// Level is an int representing the log levels.
//
// The levels are roughly model'ed around syslog.
type Level uint8

const (
	DEBUG   Level = 100 + iota // Debug or trace information.
	INFO                       // Routine information, such as ongoing status or performance
	NOTICE                     // Normal but significant events, such as start up, shut down, config
	WARNING                    // Warning events might cause problems
	ERROR                      // Error events are likely to cause problems
	SEVERE                     // Critical events cause more severe problems or brief outages

	_INVALID Level = 0
	ALWAYS   Level = 1             // Config only: specify that ALL messages are logged
	OFF      Level = math.MaxUint8 // Config only: specify that NO messages are logged
)

var level2c = map[Level]byte{
	ALWAYS:  'A',
	DEBUG:   'D',
	INFO:    'I',
	NOTICE:  'N',
	WARNING: 'W',
	ERROR:   'E',
	SEVERE:  'S',
	OFF:     'O',
}

var level2s = map[Level]string{
	ALWAYS:  "ALWAYS",
	DEBUG:   "DEBUG",
	INFO:    "INFO",
	NOTICE:  "NOTICE",
	WARNING: "WARNING",
	ERROR:   "ERROR",
	SEVERE:  "SEVERE",
	OFF:     "OFF",
}

var level4s = map[string]Level{
	"ALWAYS":  ALWAYS,
	"DEBUG":   DEBUG,
	"INFO":    INFO,
	"NOTICE":  NOTICE,
	"WARNING": WARNING,
	"ERROR":   ERROR,
	"SEVERE":  SEVERE,
	"OFF":     OFF,
}

var level4c = map[byte]Level{
	'A': ALWAYS,
	'D': DEBUG,
	'I': INFO,
	'N': NOTICE,
	'W': WARNING,
	'E': ERROR,
	'S': SEVERE,
	'O': OFF,
}

func (l Level) String() string {
	return level2s[l]
}

func (l Level) ShortString() byte {
	return level2c[l]
}

func (l Level) CodecEncodeSelf(e *codec.Encoder) {
	e.MustEncode(level2s[l])
}

func (l *Level) CodecDecodeSelf(d *codec.Decoder) {
	var s string
	d.MustDecode(&s)
	*l = level4s[s]
}

func ParseLevel(s string) (l Level) {
	return level4s[s]
}

package logging

// Level is an int representing the log levels. It typically ranges from
// ALWAYS (100) to OFF (107).
type Level uint8

const (
	INVALID Level = 0
	ALWAYS  Level = 100 + iota
	DEBUG
	INFO
	WARNING
	ERROR
	SEVERE
	OFF
)

var level2c = map[Level]byte{
	ALWAYS:  'A',
	DEBUG:   'D',
	INFO:    'I',
	WARNING: 'W',
	ERROR:   'E',
	SEVERE:  'S',
	OFF:     'O',
}

var level2s = map[Level]string{
	ALWAYS:  "ALWAYS",
	DEBUG:   "DEBUG",
	INFO:    "INFO",
	WARNING: "WARNING",
	ERROR:   "ERROR",
	SEVERE:  "SEVERE",
	OFF:     "OFF",
}

var level4s = map[string]Level{
	"ALWAYS":  ALWAYS,
	"DEBUG":   DEBUG,
	"INFO":    INFO,
	"WARNING": WARNING,
	"ERROR":   ERROR,
	"SEVERE":  SEVERE,
	"OFF":     OFF,
}

var level4c = map[byte]Level{
	'A': ALWAYS,
	'D': DEBUG,
	'I': INFO,
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

func ParseLevel(s string) (l Level) {
	return level4s[s]
}

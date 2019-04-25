package printf

import (
	"fmt"
)

const (
	debugging = true
)

func Debugf(format string, args ...interface{}) {
	if debugging {
		if len(format) == 0 || format[len(format)-1] != '\n' {
			format = format + "\n"
		}
		fmt.Printf(format, args...)
	}
}

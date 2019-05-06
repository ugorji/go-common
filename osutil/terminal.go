package osutil

import "golang.org/x/crypto/ssh/terminal"

func IsTerminal(fd int) bool {
	return terminal.IsTerminal(fd)
}

//go:build !windows

package cli

import (
	"os"
	"syscall"
)

// execReplace replaces this process with the program, as `ssh` should be.
func execReplace(path string, argv []string) error {
	return syscall.Exec(path, argv, os.Environ())
}

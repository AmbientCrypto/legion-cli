//go:build windows

package cli

import (
	"os"
	"os/exec"
)

// Windows has no exec(2): run the child and exit with its status.
func execReplace(path string, argv []string) error {
	c := exec.Command(path, argv[1:]...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil
}

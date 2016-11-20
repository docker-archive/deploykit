// +build linux darwin freebsd netbsd openbsd

package os

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// builds the logfile path given the name and args
func (l *Launcher) buildLogfile(name string, args ...string) string {
	return filepath.Join(l.logDir, fmt.Sprintf("%s-%d", name, time.Now().Unix()))
}

// returns the shell to execute the command
func getShell() *exec.Cmd {
	return exec.Command("/bin/sh")
}

// Run the command is a subshell that is detached from the parent and in background, while redirecting
// the stdout and stderr to a single log file.
func (l *Launcher) buildCmd(logfile, name string, args ...string) string {
	return fmt.Sprintf("%s %s &>%s &", name, strings.Join(args, " "), logfile)
}

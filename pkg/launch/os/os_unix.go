// +build linux darwin freebsd netbsd openbsd

package os

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
)

// builds the logfile path given the name and args
func (l *Launcher) buildLogfile(name string) string {
	return filepath.Join(l.logDir, fmt.Sprintf("%s-%d", name, time.Now().Unix()))
}

// returns the shell to execute the command
func getShell() *exec.Cmd {
	return exec.Command("/bin/sh")
}

// Run the command is a subshell that is detached from the parent and in background, while redirecting
// the stdout and stderr to a single log file.
func (l *Launcher) buildCmd(logfile, cmd string, args ...string) string {
	return fmt.Sprintf("%s %s &>%s &", cmd, strings.Join(args, " "), logfile)
}

func startAsync(name, sh string, wait chan error) {
	go func() {

		log.Infoln("OS launcher: Plugin", name, "starting", sh)
		cmd := getShell()

		// Set new pgid so the process doesn't exit when the starter exits.
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
			Pgid:    0,
		}

		defer close(wait)

		// pipe the command string to the shell
		stdin, err := cmd.StdinPipe()
		if err != nil {
			wait <- err
			return
		}

		err = cmd.Start()
		if err != nil {
			log.Warningln("OS launcher: Plugin", name, "failed to start:", err, "cmd=", sh)
			wait <- err
			return
		}
		stdin.Write([]byte(sh))
		stdin.Close()

		// TODO(chungers) - make a symbolic link to the latest log file

		wait <- cmd.Wait()
		return
	}()
}

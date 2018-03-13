// +build linux darwin freebsd netbsd openbsd

package os

import (
	"os/exec"
	"strings"
	"syscall"

	"github.com/docker/infrakit/pkg/launch"
	logutil "github.com/docker/infrakit/pkg/log"
)

var (
	log = logutil.New("module", "launch/os")
)

func start(executor launch.Exec, name, sh string, setPgID bool) <-chan error {
	block := make(chan error)

	go func() {

		defer close(block)

		log.Info("OS executor", "name", executor.Name(), "Plugin", name, "setPgId=", setPgID, "starting", sh)
		cmd := exec.Command("/bin/sh", "-c", sh)

		log.Info("Running", "cmd", cmd.Path, "args", strings.Join(cmd.Args, " "))
		// Set new pgid so the process doesn't exit when the starter exits.
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: setPgID,
		}

		err := cmd.Start()
		log.Info("Starting with", "sh", sh, "err", err)
		if err != nil {
			log.Warn("Err from OS executor", "plugin", name, "err", err, "cmd", sh)
			block <- err
		}
	}()

	return block
}

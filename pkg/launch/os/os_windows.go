package os

import (
	"os/exec"
	"strings"

	"github.com/docker/infrakit/pkg/launch"
)

func start(executor launch.Exec, name, sh string, setPgID bool) <-chan error {
	block := make(chan error)

	go func() {

		defer close(block)

		log.Info("OS executor", "name", executor.Name(), "plugin", name, "setPgId", setPgID, "cmd", sh)
		cmd := exec.Command("cmd", "/s", "/c", sh)

		log.Info("Running", cmd.Path, strings.Join(cmd.Args, " "))

		err := cmd.Start()
		log.Info("Starting with", "sh", sh, "err", err)
		if err != nil {
			log.Warn("Err from OS executor", "plugin", name, "err", err, "cmd", sh)
			block <- err
		}
	}()

	return block
}

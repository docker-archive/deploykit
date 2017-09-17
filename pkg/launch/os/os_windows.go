package os

import (
	"os/exec"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/launch"
)

func start(executor launch.Exec, name, sh string, setPgID bool) <-chan error {
	block := make(chan error)

	go func() {

		defer close(block)

		log.Infoln("OS(", executor.Name(), ") launcher: Plugin", name, "setPgId=", setPgID, "starting", sh)
		cmd := exec.Command("cmd", "/s", "/c", sh)

		log.Infoln("Running", cmd.Path, strings.Join(cmd.Args, " "))

		err := cmd.Start()
		log.Infoln("Starting with", err, "sh=", sh)
		if err != nil {
			log.Warningln("OS launcher: Plugin", name, "failed to start:", err, "cmd=", sh)
			block <- err
		}
	}()

	return block
}

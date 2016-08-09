package cli

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/libmachete/controller/util"
	"github.com/docker/libmachete/controller/util/swarm"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"os"
	"time"
)

// LeaderCmd adapts a Command to run an operation only when the local Docker Engine is a Swarm leader.
// Flags are added to the Command to control polling.  The returned func should be invoked by Command.Run or
// Command.RunE when it is ready to begin polling for leadership status.
func LeaderCmd(command cobra.Command) func(leadingWork util.RunStop) {
	var leaderOnly bool
	var dockerSocket string
	var pollInterval time.Duration

	command.Flags().BoolVar(
		&leaderOnly,
		"leader-only",
		true,
		"Run only when the local Docker Engine is a Swarm leader")
	command.Flags().StringVar(&dockerSocket, "docker-socket", "/var/run/docker.sock", "Docker daemon socket path")
	command.Flags().DurationVar(
		&pollInterval,
		"poll-interval",
		5*time.Second,
		"How often to poll for local Docker Engine leadership status")

	return func(leadingWork util.RunStop) {
		if leaderOnly {
			dockerClient, err := client.NewClient(
				fmt.Sprintf("unix://%s", dockerSocket),
				"v1.24",
				nil,
				map[string]string{})
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			err = swarm.RunWhenLeading(
				context.Background(),
				dockerClient,
				pollInterval,
				func() {
					go leadingWork.Run()
				},
				func() {
					leadingWork.Stop()
				})
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}
		} else {
			leadingWork.Run()
		}
	}
}

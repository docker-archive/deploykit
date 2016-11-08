package main

import (
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/cli"
	"github.com/docker/infrakit/discovery"
	"github.com/docker/infrakit/leader"
	"github.com/docker/infrakit/manager"
	"github.com/docker/infrakit/rpc"
	group_rpc "github.com/docker/infrakit/rpc/group"
	"github.com/docker/infrakit/store"
	"github.com/spf13/cobra"
)

type backend struct {
	id       string
	plugins  discovery.Plugins
	leader   leader.Detector
	snapshot store.Snapshot
}

func main() {

	logLevel := cli.DefaultLogLevel

	backend := &backend{
		id: "group",
	}

	// This is the name of the stateless group plugin that the manager will proxy for.
	backendName := "group-stateless"

	cmd := &cobra.Command{
		Use:   filepath.Base(os.Args[0]),
		Short: "Manager",
		PersistentPreRun: func(c *cobra.Command, args []string) {
			cli.SetLogLevel(logLevel)
		},
		PersistentPostRunE: func(c *cobra.Command, args []string) error {

			log.Infoln("Starting up manager:", backend)

			manager, err := manager.NewManager(backend.plugins,
				backend.leader, backend.snapshot, backendName)
			if err != nil {
				return err
			}

			_, err = manager.Start()
			if err != nil {
				return err
			}

			_, stopped, err := rpc.StartPluginAtPath(
				filepath.Join(discovery.Dir(), backend.id),
				group_rpc.PluginServer(manager),
				func() error {
					log.Infoln("Stopping manager")
					manager.Stop()
					return nil
				},
			)

			if err != nil {
				log.Error(err)
			}

			<-stopped // block until done
			return nil
		},
	}
	cmd.PersistentFlags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")
	cmd.PersistentFlags().StringVar(&backend.id, "name", backend.id, "Name of the manager")
	cmd.PersistentFlags().StringVar(&backendName, "proxy-for-group", backendName, "Name of the group plugin to proxy for.")

	cmd.AddCommand(cli.VersionCommand(), osEnvironment(backend), swarmEnvironment(backend))

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

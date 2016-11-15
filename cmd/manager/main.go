package main

import (
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/cli"
	"github.com/docker/infrakit/discovery"
	"github.com/docker/infrakit/leader"
	"github.com/docker/infrakit/manager"
	group_rpc "github.com/docker/infrakit/rpc/group"
	"github.com/docker/infrakit/store"
	"github.com/spf13/cobra"
)

type backend struct {
	id         string
	plugins    discovery.Plugins
	leader     leader.Detector
	snapshot   store.Snapshot
	pluginName string //This is the name of the stateless group plugin that the manager will proxy for.
}

func main() {

	logLevel := cli.DefaultLogLevel
	backend := &backend{}

	cmd := &cobra.Command{
		Use:   filepath.Base(os.Args[0]),
		Short: "Manager",
		PersistentPreRun: func(c *cobra.Command, args []string) {
			cli.SetLogLevel(logLevel)
		},
	}
	cmd.PersistentFlags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")
	cmd.PersistentFlags().StringVar(&backend.id, "name", "group", "Name of the manager")
	cmd.PersistentFlags().StringVar(&backend.pluginName, "proxy-for-group", "group-stateless", "Name of the group plugin to proxy for.")

	cmd.AddCommand(cli.VersionCommand(), osEnvironment(backend), swarmEnvironment(backend))

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func runMain(backend *backend) error {

	log.Infoln("Starting up manager:", backend)

	mgr, err := manager.NewManager(backend.plugins,
		backend.leader, backend.snapshot, backend.pluginName)
	if err != nil {
		return err
	}

	_, err = mgr.Start()
	if err != nil {
		return err
	}

	cli.RunPlugin(backend.id, group_rpc.PluginServer(mgr))

	mgr.Stop()
	log.Infoln("Manager stopped")

	return err
}

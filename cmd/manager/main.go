package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/leader"
	"github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/plugin"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	manager_rpc "github.com/docker/infrakit/pkg/rpc/manager"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/docker"
	"github.com/spf13/cobra"
)

func init() {

	cli.RegisterInfo("manager - swarm option",
		map[string]interface{}{
			"DockerClientAPIVersion": docker.ClientVersion,
		})
}

type config struct {
	id         string
	plugins    discovery.Plugins
	leader     leader.Detector
	snapshot   store.Snapshot
	pluginName string //This is the name of the stateless group plugin that the manager will proxy for.
}

func main() {

	cmd := &cobra.Command{
		Use:   filepath.Base(os.Args[0]),
		Short: "Manager",
	}

	// Log setup
	logOptions := &log.ProdDefaults

	cmd.PersistentPreRun = func(c *cobra.Command, args []string) {
		log.Configure(logOptions)
	}
	cmd.PersistentFlags().AddFlagSet(cli.Flags(logOptions))
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	pluginName := cmd.PersistentFlags().String("name", "group", "Name of the manager")
	backendPlugin := cmd.PersistentFlags().String(
		"proxy-for-group",
		"group-stateless",
		"Name of the group plugin to proxy for.")

	buildConfig := func() config {
		return config{
			id:         *pluginName,
			pluginName: *backendPlugin,
		}
	}

	cmd.AddCommand(cli.VersionCommand(),
		osEnvironment(buildConfig),
		swarmEnvironment(buildConfig),
		etcdEnvironment(buildConfig),
	)

	err := cmd.Execute()
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
}

type metadataModel struct {
	snapshot store.Snapshot
	manager  manager.Manager
}

func (m *metadataModel) pluginModel() (chan func(map[string]interface{}), chan struct{}) {
	// Start a poller to load the snapshot and make that available as metadata
	model := make(chan func(map[string]interface{}))
	stop := make(chan struct{})
	go func() {
		tick := time.Tick(1 * time.Second)
		for {
			select {
			case <-tick:
				snapshot := map[string]interface{}{}

				// update leadership
				if isLeader, err := m.manager.IsLeader(); err == nil {
					model <- func(view map[string]interface{}) {
						types.Put([]string{"leader"}, isLeader, view)
					}
				} else {
					logrus.Warningln("Cannot check leader for metadata:", err)
				}

				// update config
				if err := m.snapshot.Load(&snapshot); err == nil {
					model <- func(view map[string]interface{}) {
						types.Put([]string{"configs"}, snapshot, view)
					}
				} else {
					logrus.Warningln("Cannot load snapshot for metadata:", err)
				}

			case <-stop:
				logrus.Infoln("Snapshot updater stopped")
				return
			}
		}
	}()
	return model, stop
}

func runMain(cfg config) error {

	logrus.Infoln("Starting up manager:", cfg)

	mgr, err := manager.NewManager(cfg.plugins, cfg.leader, cfg.snapshot, cfg.pluginName)
	if err != nil {
		return err
	}

	_, err = mgr.Start()
	if err != nil {
		return err
	}

	// Start a poller to load the snapshot and make that available as metadata
	updateSnapshot := make(chan func(map[string]interface{}))
	stopSnapshot := make(chan struct{})
	go func() {
		tick := time.Tick(1 * time.Second)
		for {
			select {
			case <-tick:
				snapshot := map[string]interface{}{}

				// update leadership
				if isLeader, err := mgr.IsLeader(); err == nil {
					updateSnapshot <- func(view map[string]interface{}) {
						types.Put([]string{"leader"}, isLeader, view)
					}
				} else {
					logrus.Warningln("Cannot check leader for metadata:", err)
				}

				// update config
				if err := cfg.snapshot.Load(&snapshot); err == nil {
					updateSnapshot <- func(view map[string]interface{}) {
						types.Put([]string{"configs"}, snapshot, view)
					}
				} else {
					logrus.Warningln("Cannot load snapshot for metadata:", err)
				}

			case <-stopSnapshot:
				logrus.Infoln("Snapshot updater stopped")
				return
			}
		}
	}()

	updatable := &metadataModel{
		snapshot: cfg.snapshot,
		manager:  mgr,
	}
	updatableModel, stopUpdatable := updatable.pluginModel()
	loadFunc := func() (original *types.Any, err error) {
		var state interface{}

		if err := updatable.snapshot.Load(&state); err != nil {
			return nil, err
		}
		return types.AnyValue(state)
	}

	commitFunc := func(proposed *types.Any) error {
		newState := struct {
			Groups map[group.ID]plugin.Spec
		}{}

		if err := proposed.Decode(&newState); err != nil {
			return err
		}
		// Hacky --- there's a mismatch with how the Commit's schema and the internal
		// store's schema --> we made the map based internal representation updatable
		// so that it's possbile to use paths that contain object names (e.g. Groups/cattle/Properties as
		// opposed to Groups/0/Properties).  So here we'd have to transform the object to
		// make it the right shape.
		// leaving this code here because we will have a new schema and this will be replaced soon.
		groups := []group.Spec{}
		for _, plugin := range newState.Groups {

			spec := group.Spec{}
			if err := plugin.Properties.Decode(&spec); err != nil {
				return err
			}

			groups = append(groups, spec)
		}

		groupImpl, ok := updatable.manager.(group.Plugin)
		if !ok {
			return fmt.Errorf("manager does not implement group.Plugin interface")
		}

		for _, spec := range groups {
			if _, err := groupImpl.CommitGroup(spec, false); err != nil {
				return err
			}
		}
		return nil
	}
	cli.RunPlugin(cfg.id,
		metadata_rpc.UpdatablePluginServer(metadata_plugin.NewUpdatablePlugin(
			metadata_plugin.NewPluginFromChannel(updatableModel),
			loadFunc,
			commitFunc)),
		metadata_rpc.PluginServer(metadata_plugin.NewPluginFromChannel(updateSnapshot)),
		group_rpc.PluginServer(mgr), manager_rpc.PluginServer(mgr))

	mgr.Stop()
	close(stopSnapshot)
	close(stopUpdatable)
	logrus.Infoln("Manager stopped")

	return err
}

package main

import (
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/docker/infrakit/discovery"
	file_leader "github.com/docker/infrakit/leader/file"
	file_store "github.com/docker/infrakit/store/file"
	"github.com/spf13/cobra"
)

const (
	// LeaderFileEnvVar is the environment variable that may be used to customize the plugin leader detection
	LeaderFileEnvVar = "INFRAKIT_LEADER_FILE"

	// StoreDirEnvVar is the directory where the configs are stored
	StoreDirEnvVar = "INFRAKIT_STORE_DIR"
)

func getHome() string {
	home := os.Getenv("HOME")
	if usr, err := user.Current(); err == nil {
		home = usr.HomeDir
	}
	return home
}

func defaultLeaderFile() string {
	if leaderFile := os.Getenv(LeaderFileEnvVar); leaderFile != "" {
		return leaderFile
	}
	return filepath.Join(getHome(), ".infrakit/leader")
}

func defaultStoreDir() string {
	if storeDir := os.Getenv(StoreDirEnvVar); storeDir != "" {
		return storeDir
	}
	return filepath.Join(getHome(), ".infrakit/configs")
}

func osEnvironment(backend *backend) *cobra.Command {

	pollInterval := 5 * time.Second
	filename := defaultLeaderFile()
	storeDir := defaultStoreDir()

	cmd := &cobra.Command{
		Use:   "os",
		Short: "os",
		RunE: func(c *cobra.Command, args []string) error {

			plugins, err := discovery.NewPluginDiscovery()
			if err != nil {
				return err
			}

			leader, err := file_leader.NewDetector(pollInterval, filename, backend.id)
			if err != nil {
				return err
			}

			snapshot, err := file_store.NewSnapshot(storeDir, "global.config")
			if err != nil {
				return err
			}

			backend.plugins = plugins
			backend.leader = leader
			backend.snapshot = snapshot
			return nil
		},
	}
	cmd.Flags().StringVar(&filename, "leader-file", filename, "File used for leader election/detection")
	cmd.Flags().StringVar(&storeDir, "store-dir", storeDir, "Dir to store the config")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", pollInterval, "Leader polling interval")
	return cmd
}

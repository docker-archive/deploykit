package main

import (
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/docker/infrakit/pkg/discovery/local"
	file_leader "github.com/docker/infrakit/pkg/leader/file"
	file_store "github.com/docker/infrakit/pkg/store/file"
	"github.com/spf13/cobra"
)

const (
	// LeaderFileEnvVar is the environment variable that may be used to customize the plugin leader detection
	LeaderFileEnvVar = "INFRAKIT_LEADER_FILE"

	// StoreDirEnvVar is the directory where the configs are stored
	StoreDirEnvVar = "INFRAKIT_STORE_DIR"
)

func getHome() string {
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return os.Getenv("HOME")
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

func osEnvironment(getConfig func() config) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "os",
		Short: "os",
	}
	leaderFile := cmd.Flags().String("leader-file", defaultLeaderFile(), "File used for leader election/detection")
	storeDir := cmd.Flags().String("store-dir", defaultStoreDir(), "Dir to store the config")
	pollInterval := cmd.Flags().Duration("poll-interval", 5*time.Second, "Leader polling interval")
	cmd.RunE = func(c *cobra.Command, args []string) error {

		plugins, err := local.NewPluginDiscovery()
		if err != nil {
			return err
		}

		cfg := getConfig()
		leader, err := file_leader.NewDetector(*pollInterval, *leaderFile, cfg.id)
		if err != nil {
			return err
		}

		snapshot, err := file_store.NewSnapshot(*storeDir, "global.config")
		if err != nil {
			return err
		}

		cfg.plugins = plugins
		cfg.leader = leader
		cfg.snapshot = snapshot

		return runMain(cfg)
	}
	return cmd
}

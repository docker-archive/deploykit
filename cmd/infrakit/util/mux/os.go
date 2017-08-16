package mux

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/docker/infrakit/pkg/leader/file"
	"github.com/spf13/cobra"
)

const (
	// LeaderFileEnvVar is the environment variable that may be used to customize the plugin leader detection
	LeaderFileEnvVar = "INFRAKIT_LEADER_FILE"
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

	// if there's INFRAKIT_HOME defined
	home := os.Getenv("INFRAKIT_HOME")
	if home != "" {
		return filepath.Join(home, "leader")
	}

	return filepath.Join(getHome(), ".infrakit/leader")
}

func defaultLeaderLocationFile() string {
	if leaderFile := os.Getenv(LeaderFileEnvVar); leaderFile != "" {
		return filepath.Join(filepath.Dir(leaderFile), "leader.loc")
	}

	// if there's INFRAKIT_HOME defined
	home := os.Getenv("INFRAKIT_HOME")
	if home != "" {
		return filepath.Join(home, "leader.loc")
	}

	return filepath.Join(getHome(), ".infrakit/leader.loc")
}

func osEnvironment(cfg *config) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "os",
		Short: "os",
	}

	id := cmd.Flags().String("name", defaultLeaderFile(), "Name of this node, for matching in leader file")
	leaderFile := cmd.Flags().String("leader-file", defaultLeaderFile(), "File used for leader election/detection")
	leaderLocation := cmd.Flags().String("leader-location-file", defaultLeaderLocationFile(), "File used for storing location")

	cmd.RunE = func(c *cobra.Command, args []string) error {

		poller, err := file.NewDetector(*cfg.pollInterval, *leaderFile, *id)
		if err != nil {
			return err
		}

		cfg.poller = poller
		cfg.store = file.NewStore(*leaderLocation)

		return runMux(cfg)
	}
	return cmd
}

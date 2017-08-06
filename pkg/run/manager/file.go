package manager

import (
	"path/filepath"
	"time"

	"github.com/docker/infrakit/pkg/leader"
	file_leader "github.com/docker/infrakit/pkg/leader/file"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/store"
	file_store "github.com/docker/infrakit/pkg/store/file"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// EnvLeaderFile is the environment variable that may be used to customize the plugin leader detection
	EnvLeaderFile = "INFRAKIT_LEADER_FILE"

	// EnvStoreDir is the directory where the configs are stored
	EnvStoreDir = "INFRAKIT_STORE_DIR"
)

// BackendFileOptions contain the options for the file backend
type BackendFileOptions struct {
	// PollInterval is how often to check
	PollInterval time.Duration

	// LeaderFile is the location of the leader file
	LeaderFile string

	// StoreDir is the path to the directory where state is stored
	StoreDir string

	// ID is the id of the node
	ID string
}

// DefaultBackendFileOptions is the default for the file backend
var DefaultBackendFileOptions = Options{
	Backend: "file",
	Settings: types.AnyValueMust(
		BackendFileOptions{
			PollInterval: 5 * time.Second,
			LeaderFile:   run.GetEnv(EnvLeaderFile, filepath.Join(run.InfrakitHome(), "leader")),
			StoreDir:     run.GetEnv(EnvStoreDir, filepath.Join(run.InfrakitHome(), "configs")),
		},
	),
}

func fileBackends(options BackendFileOptions) (leader.Detector, store.Snapshot, cleanup, error) {
	leader, err := file_leader.NewDetector(options.PollInterval, options.LeaderFile, options.ID)
	if err != nil {
		return nil, nil, nil, err
	}

	snapshot, err := file_store.NewSnapshot(options.StoreDir, "global.config")
	if err != nil {
		return nil, nil, nil, err
	}

	return leader, snapshot, nil, nil
}

package manager

import (
	"fmt"
	"path/filepath"
	"time"

	file_leader "github.com/docker/infrakit/pkg/leader/file"
	"github.com/docker/infrakit/pkg/run/local"
	file_store "github.com/docker/infrakit/pkg/store/file"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// EnvLeaderFile is the environment variable that may be used to customize the plugin leader detection
	EnvLeaderFile = "INFRAKIT_LEADER_FILE"

	// EnvStoreDir is the directory where the configs are stored
	EnvStoreDir = "INFRAKIT_STORE_DIR"

	// EnvID is the id for the manager node (for file backend only)
	EnvID = "INFRAKIT_ID"
)

// BackendFileOptions contain the options for the file backend
type BackendFileOptions struct {
	// PollInterval is how often to check
	PollInterval types.Duration

	// LeaderFile is the location of the leader file
	LeaderFile string

	// StoreDir is the path to the directory where state is stored
	StoreDir string

	// ID is the id of the node
	ID string
}

// DefaultBackendFileOptions is the default for the file backend
var DefaultBackendFileOptions = BackendFileOptions{
	ID:           local.Getenv(EnvID, "manager1"),
	PollInterval: types.FromDuration(5 * time.Second),
	LeaderFile:   local.Getenv(EnvLeaderFile, filepath.Join(local.InfrakitHome(), "leader")),
	StoreDir:     local.Getenv(EnvStoreDir, filepath.Join(local.InfrakitHome(), "configs")),
}

func configFileBackends(options BackendFileOptions, managerConfig *Options) error {
	if managerConfig == nil {
		return nil
	}

	leader, err := file_leader.NewDetector(options.PollInterval.Duration(), options.LeaderFile, options.ID)
	if err != nil {
		return err
	}

	leaderStore := file_leader.NewStore(options.LeaderFile + ".loc")
	snapshot, err := file_store.NewSnapshot(options.StoreDir, "global.config")
	if err != nil {
		return err
	}

	managerConfig.Leader = leader
	managerConfig.LeaderStore = leaderStore
	managerConfig.SpecStore = snapshot

	key := "global.vars"
	if !managerConfig.Metadata.IsEmpty() {
		key = fmt.Sprintf("%s.vars", managerConfig.Metadata.Lookup())
	}

	metadataSnapshot, err := file_store.NewSnapshot(options.StoreDir, key)
	if err != nil {
		return err
	}
	managerConfig.MetadataStore = metadataSnapshot

	return nil
}

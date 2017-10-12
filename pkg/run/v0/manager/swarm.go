package manager

import (
	"time"

	"github.com/docker/go-connections/tlsconfig"
	swarm_leader "github.com/docker/infrakit/pkg/leader/swarm"
	logutil "github.com/docker/infrakit/pkg/log"
	swarm_store "github.com/docker/infrakit/pkg/store/swarm"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/docker"
)

// BackendSwarmOptions contain the options for the swarm backend
type BackendSwarmOptions struct {
	// PollInterval is how often to check
	PollInterval types.Duration
	// Docker holds the connection params to the Docker engine for join tokens, etc.
	Docker docker.ConnectInfo `json:",inline" yaml:",inline"`
}

// DefaultBackendSwarmOptions is the Options for using the swarm backend.
var DefaultBackendSwarmOptions = BackendSwarmOptions{
	PollInterval: types.FromDuration(5 * time.Second),
	Docker: docker.ConnectInfo{
		Host: "unix:///var/run/docker.sock",
		TLS:  &tlsconfig.Options{},
	},
}

func configSwarmBackends(options BackendSwarmOptions, managerConfig *Options) error {
	if managerConfig == nil {
		return nil
	}

	dockerClient, err := docker.NewClient(options.Docker.Host, options.Docker.TLS)
	log.Debug("Connect to docker", "host", options.Docker.Host, "err=", err, "V", logutil.V(100))
	if err != nil {
		return err
	}

	snapshot, err := swarm_store.NewSnapshot(dockerClient, "infrakit.specs")
	if err != nil {
		dockerClient.Close()
		return err
	}

	leader := swarm_leader.NewDetector(options.PollInterval.Duration(), dockerClient)
	leaderStore := swarm_leader.NewStore(dockerClient)

	managerConfig.Leader = leader
	managerConfig.LeaderStore = leaderStore
	managerConfig.SpecStore = snapshot
	managerConfig.cleanUpFunc = func() {
		dockerClient.Close()
		log.Debug("closed docker connection", "client", dockerClient, "V", logutil.V(100))
	}

	key := "infrakit.vars"
	if !managerConfig.Metadata.IsEmpty() {
		key = managerConfig.Metadata.Lookup()
	}

	metadataSnapshot, err := swarm_store.NewSnapshot(dockerClient, key)
	if err != nil {
		dockerClient.Close()
		return err
	}
	managerConfig.MetadataStore = metadataSnapshot

	return nil
}

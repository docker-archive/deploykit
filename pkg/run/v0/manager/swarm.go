package manager

import (
	"time"

	swarm_leader "github.com/docker/infrakit/pkg/leader/swarm"
	swarm_store "github.com/docker/infrakit/pkg/store/swarm"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/docker"
)

// BackendSwarmOptions contain the options for the swarm backend
type BackendSwarmOptions struct {
	// PollInterval is how often to check
	PollInterval time.Duration
	// Docker holds the connection params to the Docker engine for join tokens, etc.
	Docker docker.ConnectInfo `json:",inline" yaml:",inline"`
}

// DefaultBackendSwarmOptions is the Options for using the swarm backend.
var DefaultBackendSwarmOptions = Options{
	Backend: "swarm",
	Settings: types.AnyValueMust(
		BackendSwarmOptions{
			PollInterval: 5 * time.Second,
			Docker: docker.ConnectInfo{
				Host: "unix:///var/run/docker.sock",
			},
		},
	),
}

func configSwarmBackends(options BackendSwarmOptions, managerConfig *Options, muxConfig *MuxConfig) error {
	dockerClient, err := docker.NewClient(options.Docker.Host, options.Docker.TLS)
	log.Info("Connect to docker", "host", options.Docker.Host, "err=", err)
	if err != nil {
		return err
	}

	snapshot, err := swarm_store.NewSnapshot(dockerClient)
	if err != nil {
		dockerClient.Close()
		return err
	}

	leader := swarm_leader.NewDetector(options.PollInterval, dockerClient)

	if managerConfig != nil {
		managerConfig.leader = leader
		managerConfig.store = snapshot
		managerConfig.cleanUpFunc = func() { dockerClient.Close() }
	}

	if muxConfig != nil {
		muxConfig.poller = swarm_leader.NewDetector(muxConfig.PollInterval, dockerClient)
		muxConfig.store = swarm_leader.NewStore(dockerClient)
	}

	return nil
}

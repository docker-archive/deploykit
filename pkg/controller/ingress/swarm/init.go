package swarm

import (
	"fmt"

	"github.com/docker/go-connections/tlsconfig"
	ingress "github.com/docker/infrakit/pkg/controller/ingress/types"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/docker"
)

var log = logutil.New("module", "controller/ingress/swarm")

const (
	debugV  = logutil.V(300)
	debugV2 = logutil.V(310)
)

func init() {

	// Register the swarm based ingress route finder.  This will be included when the package is imported
	// in the main or wherever swarm is to be supported.
	ingress.RegisterRouteHandler(
		"swarm",
		RoutesFromSwarmServices,
	)
}

// Docker is alias for docker connection information
type Docker docker.ConnectInfo

// Spec is the struct that captures the configuration of the swarm-based ingress route finder
type Spec struct {
	// Docker holds the connection params to the Docker engine for join tokens, etc.
	Docker `json:",inline" yaml:",inline"`
}

// RoutesFromSwarmServices determines the routes based on the services running in the Docker swarm
func RoutesFromSwarmServices(properties *types.Any,
	options ingress.Options) (map[ingress.Vhost][]loadbalancer.Route, error) {

	spec := Spec{}

	err := properties.Decode(&spec)
	if err != nil {
		return nil, err
	}

	if spec.Docker.Host == "" && spec.Docker.TLS == nil {
		return nil, fmt.Errorf("no Docker connection info")
	}

	tls := spec.Docker.TLS
	if tls == nil {
		tls = &tlsconfig.Options{}
	}

	dockerClient, err := docker.NewClient(spec.Docker.Host, tls)
	if err != nil {
		return nil, err
	}

	log.Info("Connected to Docker", "client", dockerClient)
	routes, err := NewServiceRoutes(dockerClient).SetOptions(options).Build()
	if err != nil {
		return nil, err
	}

	return routes.List()
}

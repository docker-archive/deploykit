package swarm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	docker_types "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/infrakit/plugin/group/types"
	"github.com/docker/infrakit/plugin/group/util"
	"github.com/docker/infrakit/spi/flavor"
	"github.com/docker/infrakit/spi/instance"
	"github.com/magiconair/properties"
	"golang.org/x/net/context"
	"text/template"
)

type nodeType string

const (
	worker  nodeType = "worker"
	manager nodeType = "manager"
)

// NewSwarmFlavor creates a flavor.Plugin that creates manager and worker nodes connected in a swarm.
func NewSwarmFlavor(dockerClient client.APIClient) flavor.Plugin {
	return &swarmFlavor{client: dockerClient}
}

type swarmFlavor struct {
	client client.APIClient
}

type schema struct {
	Type                 nodeType
	DockerRestartCommand string
}

func parseProperties(flavorProperties json.RawMessage) (schema, error) {
	s := schema{}
	err := json.Unmarshal(flavorProperties, &s)
	return s, err
}

func (s swarmFlavor) Validate(flavorProperties json.RawMessage, allocation types.AllocationMethod) error {
	properties, err := parseProperties(flavorProperties)
	if err != nil {
		return err
	}

	if properties.DockerRestartCommand == "" {
		return errors.New("DockerRestartCommand must be specified")
	}

	switch properties.Type {
	case worker:
		return nil
	case manager:
		numIDs := len(allocation.LogicalIDs)
		if numIDs != 1 && numIDs != 3 && numIDs != 5 {
			return errors.New("Must have 1, 3, or 5 manager logical IDs")
		}

		return nil
	default:
		return errors.New("Unrecognized node Type")
	}
}

const (
	// associationTag is a machine tag added to associate machines with Swarm nodes.
	associationTag = "swarm-association-id"

	// bootScript is used to generate node boot scripts.
	bootScript = `#!/bin/sh
set -o errexit
set -o nounset
set -o xtrace

mkdir -p /etc/docker
cat << EOF > /etc/docker/daemon.json
{
  "labels": ["swarm-association-id={{.ASSOCIATION_ID}}"]
}
EOF

{{.RESTART_DOCKER}}

docker swarm join {{.MY_IP}} --token {{.JOIN_TOKEN}}
`
)

func generateInitScript(joinIP, joinToken, associationID, restartCommand string) string {
	buffer := bytes.Buffer{}
	templ := template.Must(template.New("").Parse(bootScript))
	err := templ.Execute(&buffer, map[string]string{
		"MY_IP":          joinIP,
		"JOIN_TOKEN":     joinToken,
		"ASSOCIATION_ID": associationID,
		"RESTART_DOCKER": restartCommand,
	})
	if err != nil {
		panic(err)
	}
	return buffer.String()
}

// Healthy determines whether an instance is healthy.  This is determined by whether it has successfully joined the
// Swarm.
func (s swarmFlavor) Healthy(flavorProperties json.RawMessage, inst instance.Description) (flavor.Health, error) {
	associationID, exists := inst.Tags[associationTag]
	if !exists {
		log.Info("Reporting unhealthy for instance without an association tag", inst.ID)
		return flavor.Unhealthy, nil
	}

	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=%s", associationTag, associationID))

	nodes, err := s.client.NodeList(context.Background(), docker_types.NodeListOptions{Filter: filter})
	if err != nil {
		return flavor.Unknown, err
	}

	switch {
	case len(nodes) == 0:
		// The instance may not yet be joined, so we consider the health unknown.
		return flavor.Unknown, nil

	case len(nodes) == 1:
		return flavor.Healthy, nil

	default:
		log.Warnf("Expected at most one node with label %s, but found %s", associationID, nodes)
		return flavor.Healthy, nil
	}
}

func (s swarmFlavor) Drain(flavorProperties json.RawMessage, inst instance.Description) error {
	properties, err := parseProperties(flavorProperties)
	if err != nil {
		return err
	}

	associationID, exists := inst.Tags[associationTag]
	if !exists {
		return fmt.Errorf("Unable to drain %s without an association tag", inst.ID)
	}

	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=%s", associationTag, associationID))

	nodes, err := s.client.NodeList(context.Background(), docker_types.NodeListOptions{Filter: filter})
	if err != nil {
		return flavor.Unknown, err
	}

	switch {
	case len(nodes) == 0:
		return fmt.Errorf("Unable to drain %s, not found in swarm", inst.ID)

	case len(nodes) == 1:
		// Only explicitly remove worker nodes, not manager nodes.  Manager nodes are assumed to have an
		// attached volume for state, and fixed IP addresses.  This allows them to rejoin as the same node.
		if properties.Type == worker {
			err := s.client.NodeRemove(
				context.Background(),
				nodes[0].ID,
				docker_types.NodeRemoveOptions{Force: true})
			if err != nil {
				return err
			}
		}

		return nil

	default:
		return fmt.Errorf("Expected at most one node with label %s, but found %s", associationID, nodes)
	}
}

func (s swarmFlavor) Prepare(
	flavorProperties json.RawMessage,
	spec instance.Spec,
	allocation types.AllocationMethod) (instance.Spec, error) {

	properties, err := parseProperties(flavorProperties)
	if err != nil {
		return spec, err
	}

	swarmStatus, err := s.client.SwarmInspect(context.Background())
	if err != nil {
		return spec, fmt.Errorf("Failed to fetch Swarm join tokens: %s", err)
	}

	nodeInfo, err := s.client.Info(context.Background())
	if err != nil {
		return spec, fmt.Errorf("Failed to fetch node self info: %s", err)
	}

	self, _, err := s.client.NodeInspectWithRaw(context.Background(), nodeInfo.Swarm.NodeID)
	if err != nil {
		return spec, fmt.Errorf("Failed to fetch Swarm node status: %s", err)
	}

	if self.ManagerStatus == nil {
		return spec, errors.New(
			"Swarm node status did not include manager status.  Need to run 'docker swarm init`?")
	}

	associationID := util.RandomAlphaNumericString(8)
	spec.Tags[associationTag] = associationID

	switch properties.Type {
	case worker:
		spec.Init = generateInitScript(
			self.ManagerStatus.Addr,
			swarmStatus.JoinTokens.Worker,
			associationID,
			properties.DockerRestartCommand)

	case manager:
		if spec.LogicalID == nil {
			return spec, errors.New("Manager nodes require an assigned private IP address")
		}

		spec.Init = generateInitScript(
			self.ManagerStatus.Addr,
			swarmStatus.JoinTokens.Manager,
			associationID,
			properties.DockerRestartCommand)

		spec.Attachments = []instance.Attachment{instance.Attachment(*spec.LogicalID)}

	default:
		return spec, errors.New("Unsupported node type")
	}

	// TODO(wfarner): Use the cluster UUID to scope instances for this swarm separately from instances in another
	// swarm.  This will require plumbing back to Scaled (membership tags).
	spec.Tags["swarm-id"] = swarmStatus.ID

	return spec, nil
}

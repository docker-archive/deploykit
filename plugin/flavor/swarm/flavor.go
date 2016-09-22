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
	"github.com/docker/libmachete/plugin/group/types"
	"github.com/docker/libmachete/plugin/group/util"
	"github.com/docker/libmachete/spi/flavor"
	"github.com/docker/libmachete/spi/instance"
	"golang.org/x/net/context"
	"text/template"
)

const (
	roleWorker  = "worker"
	roleManager = "manager"
)

// NewSwarmFlavor creates a flavor.Plugin that creates manager and worker nodes connected in a swarm.
func NewSwarmFlavor(dockerClient client.APIClient) flavor.Plugin {
	return &swarmProvisioner{client: dockerClient}
}

type swarmProvisioner struct {
	client client.APIClient
}

// TODO(wfarner): Add a ProvisionHelper function to check the health of an instance.  Use the Swarm node association
// (see TODO above) to provide this.

func nodeTypeFromProperties(flavorProperties json.RawMessage) (string, error) {
	properties := map[string]string{}

	if err := json.Unmarshal(flavorProperties, &properties); err != nil {
		return "", err
	}

	return properties["type"], nil
}

func (s swarmProvisioner) Validate(flavorProperties json.RawMessage, parsed types.Schema) (flavor.InstanceIDKind, error) {
	nodeType, err := nodeTypeFromProperties(flavorProperties)
	if err != nil {
		return flavor.IDKindUnknown, err
	}

	if nodeType == roleManager {
		if len(parsed.LogicalIDs) != 1 && len(parsed.LogicalIDs) != 3 && len(parsed.LogicalIDs) != 5 {
			return flavor.IDKindUnknown, errors.New("Must have 1, 3, or 5 managers")
		}
	}

	switch nodeType {
	case roleWorker:
		return flavor.IDKindPhysical, nil
	case roleManager:
		return flavor.IDKindPhysicalWithLogical, nil
	default:
		return flavor.IDKindUnknown, nil
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

start_install() {
  if command -v docker >/dev/null
  then
    echo 'Detected existing Docker installation, will not attempt to install or update'
  else
    sleep 5
    wget -qO- https://get.docker.com/ | sh
  fi

  docker swarm join {{.MY_IP}} --token {{.JOIN_TOKEN}}
}

# See https://github.com/docker/docker/issues/23793#issuecomment-237735835 for
# details on why we background/sleep.
start_install &
`
)

func generateInitScript(joinIP, joinToken, associationID string) string {
	buffer := bytes.Buffer{}
	templ := template.Must(template.New("").Parse(bootScript))
	err := templ.Execute(&buffer, map[string]string{
		"MY_IP":          joinIP,
		"JOIN_TOKEN":     joinToken,
		"ASSOCIATION_ID": associationID,
	})
	if err != nil {
		panic(err)
	}
	return buffer.String()
}

// Healthy determines whether an instance is healthy.  This is determined by whether it has successfully joined the
// Swarm.
func (s swarmProvisioner) Healthy(inst instance.Description) (bool, error) {
	associationID, exists := inst.Tags[associationTag]
	if !exists {
		log.Info("Reporting unhealthy for instance without an association tag", inst.ID)
		return false, nil
	}

	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=%s", associationTag, associationID))

	nodes, err := s.client.NodeList(context.Background(), docker_types.NodeListOptions{Filter: filter})
	if err != nil {
		return false, err
	}

	if len(nodes) > 1 {
		log.Warnf("Expected at most one node with label %s, but found %s", associationID, nodes)
	}

	// If a node was returned from the query, the association ID is present and the node is healthy.
	return len(nodes) == 1, nil
}

func (s swarmProvisioner) PreProvision(flavorProperties json.RawMessage, spec instance.Spec) (instance.Spec, error) {

	nodeType, err := nodeTypeFromProperties(flavorProperties)
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

	switch nodeType {
	case roleWorker:
		spec.Init = generateInitScript(
			self.ManagerStatus.Addr,
			swarmStatus.JoinTokens.Worker,
			associationID)

	case roleManager:
		if spec.LogicalID == nil {
			return spec, errors.New("Manager nodes require an assigned private IP address")
		}

		spec.Init = generateInitScript(
			self.ManagerStatus.Addr,
			swarmStatus.JoinTokens.Manager,
			associationID)

		spec.Attachments = []instance.Attachment{instance.Attachment(*spec.LogicalID)}

	default:
		return spec, errors.New("Unsupported role type")
	}

	// TODO(wfarner): Use the cluster UUID to scope instances for this swarm separately from instances in another
	// swarm.  This will require plumbing back to Scaled (membership tags).
	spec.Tags["swarm-id"] = swarmStatus.ID

	return spec, nil
}

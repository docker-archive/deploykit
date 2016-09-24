package zookeeper

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/docker/libmachete/plugin/group/types"
	"github.com/docker/libmachete/spi/flavor"
	"github.com/docker/libmachete/spi/instance"
	"text/template"
)

const (
	roleMember = "member"
)

// NewPlugin creates a ProvisionHelper that creates manager and worker nodes connected in a swarm.
func NewPlugin() flavor.Plugin {
	return &swarmProvisioner{}
}

type swarmProvisioner struct {
}

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

	switch nodeType {
	case roleMember:
		return flavor.IDKindPhysicalWithLogical, nil
	default:
		return flavor.IDKindUnknown, nil
	}
}

const (
	// bootScript is used to generate node boot scripts.
	bootScript = `#!/bin/sh
set -o errexit
set -o nounset
set -o xtrace

mkdir -p /etc/zookeeper/conf
cat << EOF > /etc/zookeeper/conf.zoo.cfg
tickTime=2000
dataDir=/var/zookeeper
clientPort=2181
initLimit=5
syncLimit=2
{{range $i, $ip := .Servers}}
server.{{$i}}={{$ip}}:2888:3888
{{end}}
EOF

apt-get update
apt-get install -y zookeeperd
`
)

func generateBootScript(servers []string) string {
	buffer := bytes.Buffer{}
	templ := template.Must(template.New("").Parse(bootScript))
	if err := templ.Execute(&buffer, map[string]interface{}{"Servers": servers}); err != nil {
		panic(err)
	}
	return buffer.String()
}

// Healthy determines whether an instance is healthy.  This is determined by whether it has successfully joined the
// Swarm.
func (s swarmProvisioner) Healthy(inst instance.Description) (bool, error) {
	return true, nil
}

func (s swarmProvisioner) PreProvision(flavorProperties json.RawMessage, spec instance.Spec) (instance.Spec, error) {
	nodeType, err := nodeTypeFromProperties(flavorProperties)
	if err != nil {
		return spec, err
	}

	switch nodeType {
	case roleMember:
		if spec.LogicalID == nil {
			return spec, errors.New("Manager nodes require an assigned private IP address")
		}

		// TODO(wfarner): Need access to the parsed group properties.
		spec.Init = generateBootScript([]string{string(*spec.LogicalID)})

	default:
		return spec, errors.New("Unsupported role type")
	}

	return spec, nil
}

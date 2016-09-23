package zookeeper

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/docker/libmachete/spi/flavor"
	"github.com/docker/libmachete/spi/instance"
	"text/template"
)

const (
	roleMember = "member"
)

// NewPlugin creates a ProvisionHelper that creates manager and worker nodes connected in a ZooKeeper quorum.
func NewPlugin() flavor.Plugin {
	return &zkFlavor{}
}

type zkFlavor struct {
}

type schema struct {
	Type string
	Size uint
	IPs  []instance.LogicalID
}

func parseProperties(flavorProperties json.RawMessage) (schema, error) {
	s := schema{}
	err := json.Unmarshal(flavorProperties, &s)
	return s, err
}

func (s zkFlavor) Validate(flavorProperties json.RawMessage) (flavor.AllocationMethod, error) {
	properties, err := parseProperties(flavorProperties)
	if err != nil {
		return flavor.AllocationMethod{}, err
	}

	switch properties.Type {
	case roleMember:
		return flavor.AllocationMethod{LogicalIDs: properties.IPs}, nil
	default:
		return flavor.AllocationMethod{}, errors.New("Unrecognized node Type")
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

// Healthy determines whether an instance is healthy.
func (s zkFlavor) Healthy(inst instance.Description) (bool, error) {
	// TODO(wfarner): Implement.
	return true, nil
}

func (s zkFlavor) Prepare(flavorProperties json.RawMessage, spec instance.Spec) (instance.Spec, error) {
	properties, err := parseProperties(flavorProperties)
	if err != nil {
		return spec, err
	}

	switch properties.Type {
	case roleMember:
		if spec.LogicalID == nil {
			return spec, errors.New("Manager nodes require an assigned private IP address")
		}

		// TODO(wfarner): Use the node ID's position within schema.IPs as the myid value.
		spec.Init = generateBootScript([]string{string(*spec.LogicalID)})

	default:
		return spec, errors.New("Unsupported role type")
	}

	return spec, nil
}

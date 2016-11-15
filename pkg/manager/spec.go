package manager

import (
	"encoding/json"

	"github.com/docker/infrakit/pkg/spi/group"
)

// GlobalSpec is a simple model of a collection of Group plugin configs
type GlobalSpec struct {
	Groups map[group.ID]PluginSpec
}

// PluginSpec is a standard representation of a Plugin that specifies it by name and custom properties
type PluginSpec struct {
	Plugin     string
	Properties *json.RawMessage
}

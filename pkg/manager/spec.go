package manager

import (
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
)

// globalSpec is a simple model of a collection of Group plugin configs
type globalSpec struct {
	Groups map[group.ID]plugin.Spec
}

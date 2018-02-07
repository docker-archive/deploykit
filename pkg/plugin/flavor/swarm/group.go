package swarm

import (
	"fmt"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// NewGroupPlugin creates the swarm group plugin
func NewGroupPlugin(instPlugin instance.Plugin) group.Plugin {
	return &GroupPlugin{
		instPlugin: instPlugin,
	}
}

// GroupPlugin is the group plugin, with the nested swarm instance plugin
type GroupPlugin struct {
	instPlugin instance.Plugin
}

// DescribeGroup .
func (s *GroupPlugin) DescribeGroup(id group.ID) (group.Description, error) {
	insts, err := s.instPlugin.DescribeInstances(map[string]string{}, true)
	if err != nil {
		return group.Description{}, err
	}
	result := group.Description{}
	// Filter on "ready" nodes that contain an infrakit-link (which is the logical ID)
	for _, i := range insts {
		if i.LogicalID == nil {
			log.Warn("DescribeSwarmGroup", "Ignoring swarm node, missing LogicalID", i)
			continue
		}
		node := swarm.Node{}
		err := i.Properties.Decode(&node)
		if err != nil {
			log.Error("DescribeSwarmGroup", "Failed to decode swarm node", i)
			return group.Description{}, err
		}
		state := node.Status.State
		if state != swarm.NodeStateReady {
			log.Info("DescribeSwarmGroup", "Ignoring non-ready swarm node, state", state, "node", node.Description.Hostname)
			continue
		}
		result.Instances = append(result.Instances, i)
	}
	return result, nil
}

// Size .
func (s *GroupPlugin) Size(id group.ID) (int, error) {
	desc, err := s.DescribeGroup(id)
	if err != nil {
		return 0, err
	}
	return len(desc.Instances), nil
}

// CommitGroup is not suported
func (s *GroupPlugin) CommitGroup(grp group.Spec, pretend bool) (string, error) {
	return "", fmt.Errorf("CommitGroup not supported for swarm group")
}

// FreeGroup is not suported
func (s *GroupPlugin) FreeGroup(group.ID) error {
	return fmt.Errorf("FreeGroup not supported for swarm group")
}

// InspectGroups is not suported
func (s *GroupPlugin) InspectGroups() ([]group.Spec, error) {
	return []group.Spec{}, fmt.Errorf("InspectGroups not supported for swarm group")
}

// DestroyGroup is not suported
func (s *GroupPlugin) DestroyGroup(group.ID) error {
	return fmt.Errorf("DestroyGroup not supported for swarm group")
}

// DestroyInstances is not suported
func (s *GroupPlugin) DestroyInstances(group.ID, []instance.ID) error {
	return fmt.Errorf("DestroyInstances not supported for swarm group")
}

// SetSize is not suported
func (s *GroupPlugin) SetSize(group.ID, int) error {
	return fmt.Errorf("SetSize not supported for swarm group")
}

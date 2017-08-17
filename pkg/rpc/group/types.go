package group

import (
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// CommitGroupRequest is the rpc wrapper for input to commit a group
type CommitGroupRequest struct {
	Spec    group.Spec
	Pretend bool
}

// CommitGroupResponse is the rpc wrapper for the results to commit a group
type CommitGroupResponse struct {
	ID      group.ID
	Details string
}

// FreeGroupRequest is the rpc wrapper for input to free a group
type FreeGroupRequest struct {
	ID group.ID
}

// FreeGroupResponse is the rpc wrapper for the results to free a group
type FreeGroupResponse struct {
	ID group.ID
}

// DescribeGroupRequest is the rpc wrapper for the input to inspect a group
type DescribeGroupRequest struct {
	ID group.ID
}

// DescribeGroupResponse is the rpc wrapper for the results from inspecting a group
type DescribeGroupResponse struct {
	ID          group.ID
	Description group.Description
}

// DestroyGroupRequest is the rpc wrapper for the input to destroy a group
type DestroyGroupRequest struct {
	ID group.ID
}

// DestroyGroupResponse is the rpc wrapper for the output from destroying a group
type DestroyGroupResponse struct {
	ID group.ID
}

// InspectGroupsRequest is the rpc wrapper for the input to inspect groups
type InspectGroupsRequest struct {
	ID group.ID
}

// InspectGroupsResponse is the rpc wrapper for the output from inspecting groups
type InspectGroupsResponse struct {
	ID     group.ID
	Groups []group.Spec
}

// DestroyInstancesRequest is the rpc wrapper for the input to destroy instances
type DestroyInstancesRequest struct {
	ID        group.ID
	Instances []instance.ID
}

// DestroyInstancesResponse is the rpc wrapper for the output from destroy instances
type DestroyInstancesResponse struct {
	ID group.ID
}

// SizeRequest is the rpc wrapper for the getting size
type SizeRequest struct {
	ID group.ID
}

// SizeResponse is the rpc wrapper for the output of sizefrom destroy instances
type SizeResponse struct {
	ID   group.ID
	Size int
}

// SetSizeRequest is the rpc wrapper for the getting size
type SetSizeRequest struct {
	ID   group.ID
	Size int
}

// SetSizeResponse is the rpc wrapper for the output of sizefrom destroy instances
type SetSizeResponse struct {
	ID group.ID
}

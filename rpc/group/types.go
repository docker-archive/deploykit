package group

import (
	"github.com/docker/infrakit/spi/group"
)

// WatchGroupRequest is the rpc wrapper for input to watch a group
type WatchGroupRequest struct {
	Spec group.Spec
}

// WatchGroupResponse is the rpc wrapper for the results to watch a group
type WatchGroupResponse struct {
	OK bool
}

// UnwatchGroupRequest is the rpc wrapper for input to unwatch a group
type UnwatchGroupRequest struct {
	ID group.ID
}

// UnwatchGroupResponse is the rpc wrapper for the results to unwatch a group
type UnwatchGroupResponse struct {
	OK bool
}

// DescribeGroupRequest is the rpc wrapper for the input to inspect a group
type DescribeGroupRequest struct {
	ID group.ID
}

// DescribeGroupResponse is the rpc wrapper for the results from inspecting a group
type DescribeGroupResponse struct {
	Description group.Description
}

// DescribeUpdateRequest is the rpc wrapper for the input to describe an update
type DescribeUpdateRequest struct {
	Spec group.Spec
}

// DescribeUpdateResponse is the rpc wrapper for the results from describing an update
type DescribeUpdateResponse struct {
	Plan string
}

// UpdateGroupRequest is the rpc wrapper for the input to update a group
type UpdateGroupRequest struct {
	Spec group.Spec
}

// UpdateGroupResponse is the rpc wrapper for the results of updating a group
type UpdateGroupResponse struct {
	OK bool
}

// StopUpdateRequest is the rpc wrapper for input to stop an update
type StopUpdateRequest struct {
	ID group.ID
}

// StopUpdateResponse is the rpc wrapper for the output from stopping an update
type StopUpdateResponse struct {
	OK bool
}

// DestroyGroupRequest is the rpc wrapper for the input to destroy a group
type DestroyGroupRequest struct {
	ID group.ID
}

// DestroyGroupResponse is the rpc wrapper for the output from destroying a group
type DestroyGroupResponse struct {
	OK bool
}

// InspectGroupsRequest is the rpc wrapper for the input to destroy a group
type InspectGroupsRequest struct {
	ID group.ID
}

// InspectGroupsResponse is the rpc wrapper for the output from destroying a group
type InspectGroupsResponse struct {
	Groups []group.Spec
}

// RPCService is the interface for exposing the group plugin as a RPC service. It conforms to the call conventions
// defined in net/rpc
type RPCService interface {
	WatchGroup(req *WatchGroupRequest, resp *WatchGroupResponse) error
	UnwatchGroup(req *UnwatchGroupRequest, resp *UnwatchGroupResponse) error
	DescribeGroup(req *DescribeGroupRequest, resp *DescribeGroupResponse) error
	DescribeUpdate(req *DescribeUpdateRequest, resp *DescribeUpdateResponse) error
	UpdateGroup(req *UpdateGroupRequest, resp *UpdateGroupResponse) error
	StopUpdate(req *StopUpdateRequest, resp *StopUpdateResponse) error
	DestroyGroup(req *DestroyGroupRequest, resp *DestroyGroupResponse) error
	InspectGroups(req *InspectGroupsRequest, resp *InspectGroupsResponse) error
}

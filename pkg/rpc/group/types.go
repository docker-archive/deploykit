package group

import (
	"fmt"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// CommitGroupRequest is the rpc wrapper for input to commit a group
type CommitGroupRequest struct {
	Spec    group.Spec
	Pretend bool
}

// Plugin implements pkg/rpc/internal/Addressable
func (r CommitGroupRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.Spec.ID)), nil
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

// Plugin implements pkg/rpc/internal/Addressable
func (r FreeGroupRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.ID)), nil
}

// FreeGroupResponse is the rpc wrapper for the results to free a group
type FreeGroupResponse struct {
	ID group.ID
}

// DescribeGroupRequest is the rpc wrapper for the input to inspect a group
type DescribeGroupRequest struct {
	ID group.ID
}

// Plugin implements pkg/rpc/internal/Addressable
func (r DescribeGroupRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.ID)), nil
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

// Plugin implements pkg/rpc/internal/Addressable
func (r DestroyGroupRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.ID)), nil
}

// DestroyGroupResponse is the rpc wrapper for the output from destroying a group
type DestroyGroupResponse struct {
	ID group.ID
}

// InspectGroupsRequest is the rpc wrapper for the input to inspect groups
type InspectGroupsRequest struct {
	ID group.ID
}

// Plugin implements pkg/rpc/internal/Addressable
func (r InspectGroupsRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.ID)), nil
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

// Plugin implements pkg/rpc/internal/Addressable
func (r DestroyInstancesRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.ID)), nil
}

// DestroyInstancesResponse is the rpc wrapper for the output from destroy instances
type DestroyInstancesResponse struct {
	ID group.ID
}

// SizeRequest is the rpc wrapper for the getting size
type SizeRequest struct {
	ID group.ID
}

// Plugin implements pkg/rpc/internal/Addressable
func (r SizeRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.ID)), nil
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

// Plugin implements pkg/rpc/internal/Addressable
func (r SetSizeRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.ID)), nil
}

// SetSizeResponse is the rpc wrapper for the output of sizefrom destroy instances
type SetSizeResponse struct {
	ID group.ID
}

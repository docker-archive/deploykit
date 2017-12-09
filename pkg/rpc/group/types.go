package group

import (
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// CommitGroupRequest is the rpc wrapper for input to commit a group
type CommitGroupRequest struct {
	Name    plugin.Name
	Spec    group.Spec
	Pretend bool
}

// Plugin implements pkg/rpc/internal/Addressable
func (r CommitGroupRequest) Plugin() (plugin.Name, error) {
	return r.Name, nil
}

// CommitGroupResponse is the rpc wrapper for the results to commit a group
type CommitGroupResponse struct {
	Name    plugin.Name
	ID      group.ID
	Details string
}

// FreeGroupRequest is the rpc wrapper for input to free a group
type FreeGroupRequest struct {
	Name plugin.Name
	ID   group.ID
}

// Plugin implements pkg/rpc/internal/Addressable
func (r FreeGroupRequest) Plugin() (plugin.Name, error) {
	return r.Name, nil
}

// FreeGroupResponse is the rpc wrapper for the results to free a group
type FreeGroupResponse struct {
	Name plugin.Name
	ID   group.ID
}

// DescribeGroupRequest is the rpc wrapper for the input to inspect a group
type DescribeGroupRequest struct {
	Name plugin.Name
	ID   group.ID
}

// Plugin implements pkg/rpc/internal/Addressable
func (r DescribeGroupRequest) Plugin() (plugin.Name, error) {
	return r.Name, nil
}

// DescribeGroupResponse is the rpc wrapper for the results from inspecting a group
type DescribeGroupResponse struct {
	Name        plugin.Name
	ID          group.ID
	Description group.Description
}

// DestroyGroupRequest is the rpc wrapper for the input to destroy a group
type DestroyGroupRequest struct {
	Name plugin.Name
	ID   group.ID
}

// Plugin implements pkg/rpc/internal/Addressable
func (r DestroyGroupRequest) Plugin() (plugin.Name, error) {
	return r.Name, nil
}

// DestroyGroupResponse is the rpc wrapper for the output from destroying a group
type DestroyGroupResponse struct {
	Name plugin.Name
	ID   group.ID
}

// InspectGroupsRequest is the rpc wrapper for the input to inspect groups
type InspectGroupsRequest struct {
	Name plugin.Name
	ID   group.ID
}

// Plugin implements pkg/rpc/internal/Addressable
func (r InspectGroupsRequest) Plugin() (plugin.Name, error) {
	return r.Name, nil
}

// InspectGroupsResponse is the rpc wrapper for the output from inspecting groups
type InspectGroupsResponse struct {
	Name   plugin.Name
	ID     group.ID
	Groups []group.Spec
}

// DestroyInstancesRequest is the rpc wrapper for the input to destroy instances
type DestroyInstancesRequest struct {
	Name      plugin.Name
	ID        group.ID
	Instances []instance.ID
}

// Plugin implements pkg/rpc/internal/Addressable
func (r DestroyInstancesRequest) Plugin() (plugin.Name, error) {
	return r.Name, nil
}

// DestroyInstancesResponse is the rpc wrapper for the output from destroy instances
type DestroyInstancesResponse struct {
	Name plugin.Name
	ID   group.ID
}

// SizeRequest is the rpc wrapper for the getting size
type SizeRequest struct {
	Name plugin.Name
	ID   group.ID
}

// Plugin implements pkg/rpc/internal/Addressable
func (r SizeRequest) Plugin() (plugin.Name, error) {
	return r.Name, nil
}

// SizeResponse is the rpc wrapper for the output of sizefrom destroy instances
type SizeResponse struct {
	Name plugin.Name
	ID   group.ID
	Size int
}

// SetSizeRequest is the rpc wrapper for the getting size
type SetSizeRequest struct {
	Name plugin.Name
	ID   group.ID
	Size int
}

// Plugin implements pkg/rpc/internal/Addressable
func (r SetSizeRequest) Plugin() (plugin.Name, error) {
	return r.Name, nil
}

// SetSizeResponse is the rpc wrapper for the output of sizefrom destroy instances
type SetSizeResponse struct {
	Name plugin.Name
	ID   group.ID
}

package group

import (
	"github.com/docker/infrakit/spi/group"
)

type WatchGroupRequest struct {
	Spec group.Spec
}

type WatchGroupResponse struct {
	OK bool
}

type UnwatchGroupRequest struct {
	ID group.ID
}

type UnwatchGroupResponse struct {
	OK bool
}

type InspectGroupRequest struct {
	ID group.ID
}

type InspectGroupResponse struct {
	Description group.Description
}

type DescribeUpdateRequest struct {
	Spec group.Spec
}

type DescribeUpdateResponse struct {
	Plan string
}

type UpdateGroupRequest struct {
	Spec group.Spec
}

type UpdateGroupResponse struct {
	OK bool
}

type StopUpdateRequest struct {
	ID group.ID
}

type StopUpdateResponse struct {
	OK bool
}

type DestroyGroupRequest struct {
	ID group.ID
}

type DestroyGroupResponse struct {
	OK bool
}

type RPCService interface {
	WatchGroup(req *WatchGroupRequest, resp *WatchGroupResponse) error
	UnwatchGroup(req *UnwatchGroupRequest, resp *UnwatchGroupResponse) error
	InspectGroup(req *InspectGroupRequest, resp *InspectGroupResponse) error
	DescribeUpdate(req *DescribeUpdateRequest, resp *DescribeUpdateResponse) error
	UpdateGroup(req *UpdateGroupRequest, resp *UpdateGroupResponse) error
	StopUpdate(req *StopUpdateRequest, resp *StopUpdateResponse) error
	DestroyGroup(req *DestroyGroupRequest, resp *DestroyGroupResponse) error
}

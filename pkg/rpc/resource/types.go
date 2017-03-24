package resource

import "github.com/docker/infrakit/pkg/spi/resource"

// CommitRequest is the RPC wrapper for the Commit request.
type CommitRequest struct {
	Spec    resource.Spec
	Pretend bool
}

// CommitResponse is the RPC wrapper for the Commit response.
type CommitResponse struct {
	Details string
}

// DestroyRequest is the RPC wrapper for the Destroy request.
type DestroyRequest struct {
	Spec    resource.Spec
	Pretend bool
}

// DestroyResponse is the RPC wrapper for the Destroy response.
type DestroyResponse struct {
	Details string
}

// DescribeResourcesRequest is the RPC wrapper for the DescribeResources request.
type DescribeResourcesRequest struct {
	Spec resource.Spec
}

// DescribeResourcesResponse is the RPC wrapper for the DescribeResources response.
type DescribeResourcesResponse struct {
	Details string
}

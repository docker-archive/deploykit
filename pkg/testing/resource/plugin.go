package resource

import "github.com/docker/infrakit/pkg/spi/resource"

// Plugin implements resource.Plugin.
type Plugin struct {

	// DoCommit implements Commit.
	DoCommit func(spec resource.Spec, pretend bool) (string, error)

	// DoDestroy implements Destroy.
	DoDestroy func(spec resource.Spec, pretend bool) (string, error)

	// DoDescribeResources implements DescribeResources.
	DoDescribeResources func(spec resource.Spec) (string, error)
}

// Commit creates the resources in the spec that do not exist.
func (t *Plugin) Commit(spec resource.Spec, pretend bool) (string, error) {
	return t.DoCommit(spec, pretend)
}

// Destroy destroys all resources in the spec.
func (t *Plugin) Destroy(spec resource.Spec, pretend bool) (string, error) {
	return t.DoDestroy(spec, pretend)
}

// DescribeResources describes the resources in the spec that exist.
func (t *Plugin) DescribeResources(spec resource.Spec) (string, error) {
	return t.DoDescribeResources(spec)
}

package core

import (
	"fmt"

	"github.com/docker/infrakit/pkg/types"
)

type errBadDependency types.Dependency

func (e errBadDependency) Error() string {
	return fmt.Sprintf("unresolved dependency: class=%s name=%s", types.Dependency(e).Class, types.Dependency(e).Name)
}

type errCircularDependency []*types.Spec

func (e errCircularDependency) Error() string {
	deps := []*types.Spec(e)
	list := fmt.Sprintf("%s/%s", deps[0].Class, deps[0].Metadata.Name)
	for _, dep := range deps[1:] {
		list = list + fmt.Sprintf("=> %s/%s", dep.Class, dep.Metadata.Name)
	}
	return fmt.Sprintf("circular dependency: %s", list)
}

type errNotFound struct {
	class string
	name  string
}

func (e errNotFound) Error() string {
	return fmt.Sprintf("not found %s/%s", e.class, e.name)
}

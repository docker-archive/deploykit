package types

import (
	"fmt"
)

type errMissingAttribute string

func (e errMissingAttribute) Error() string {
	return fmt.Sprintf("missing attribute: %s", string(e))
}

type errBadDependency Dependency

func (e errBadDependency) Error() string {
	return fmt.Sprintf("unresolved dependency: class=%s name=%s", Dependency(e).Class, Dependency(e).Name)
}

type errCircularDependency []*Spec

func (e errCircularDependency) Error() string {
	deps := []*Spec(e)
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

package plugin

import (
	"fmt"
	"strings"

	"github.com/docker/infrakit/pkg/types"
)

/*
Examples:
{ kind:ingress,          name:lb1 }             => { kind:ingress,   plugin:ingress/lb1,          instance: lb1 }
{ kind:ingress,          name:us-east/lb1 }     => { kind:ingress,   plugin:us-east/lb1,          instance: lb1 }
{ kind:group,            name:workers }         => { kind:group,     plugin:group/workers,        instance: workers }
{ kind:group,            name:us-east/workers } => { kind:group,     plugin:us-east/workers,      instance: workers }
{ kind:resource,         name:vpc1 }            => { kind:resource,  plugin:resource/vpc1,        instance: vpc1  }
{ kind:resource,         name:us-east/vpc1 }    => { kind:resource,  plugin:us-east/vpc1,         instance: vpc1  }
{ kind:simulator/disk,   name:disk1 }           => { kind:simulator, plugin:simulator/disk,       instance: disk1 }
{ kind:simulator/disk,   name:us-east/disk1 }   => { kind:simulator, plugin:us-east1/disk         instance: disk1 }
{ kind:aws/ec2-instance, name:host1 }           => { kind:aws,       plugin:aws/ec2-instance      instance: host1 }
{ kind:aws/ec2-instance, name:us-east/host1 }   => { kind:aws,       plugin:us-east1/ec2-instance instance: host1 }
*/

// Addressable represents an object that can be reached / located, whether it's an RPC endpoint
// or an object within the rpc endpoint.
type Addressable interface {
	// Kind corresponds to the packages under pkg/run/v0
	Kind() string
	// Plugin returns the address of the rpc (endpoint)
	Plugin() Name
	// Instance is the instance identifier
	Instance() string
	// String returns the string representation for debugging
	String() string
}

// NewAddressableFromPluginMetadata returns an Addressable from the plugin metadata
func NewAddressableFromPluginMetadata(meta Metadata) Addressable {
	return NewAddressable(meta.Kind, meta.Name, meta.Instance)
}

// NewAddressableFromMetadata creates an addressable from metadata
func NewAddressableFromMetadata(kind string, metadata types.Metadata) Addressable {
	instance := ""
	if metadata.Identity != nil {
		instance = metadata.Identity.ID
	}
	return NewAddressable(kind, Name(metadata.Name), instance)
}

// NewAddressableFromPluginName returns a generic addressable object from just the plugin name.
// The kind is assume to be the same as the lookup.
func NewAddressableFromPluginName(pn Name) Addressable {
	return NewAddressable(pn.Lookup(), pn, "")
}

// NewAddressable returns a generic addressable object
func NewAddressable(kind string, pn Name, instance string) Addressable {
	n := string(pn)
	endsWithSlash := n[len(n)-1] == '/'

	lookup, sub := pn.GetLookupAndType()
	if sub == "" {
		sub = instance
	}
	name := pn
	if sub != instance {
		// Use the a different name only when we changed the subtype
		name = NameFrom(lookup, sub)
	}
	if endsWithSlash && instance != "" {
		// If the name ended with / but instance is specified, then qualify it
		name = NameFrom(lookup, instance)
	}

	spec := types.Spec{
		Kind: kind,
		Metadata: types.Metadata{
			Name: string(name),
		},
	}
	if instance != "" {
		spec.Metadata.Identity = &types.Identity{ID: instance}
	}
	return AsAddressable(spec)
}

// AsAddressable returns a spec as an addressable object
func AsAddressable(spec types.Spec) Addressable {
	a := &specQuery{Spec: spec}
	a.Plugin() // initializes
	return a
}

type specQuery struct {
	types.Spec
	instance string // derived
}

// Kind returns the kind to use for launching.  It's assumed these map to something in the launch Rules.
func (ps specQuery) Kind() string {
	// kind can be qualified, like aws/ec2-instance, but the kind is always the base.
	return strings.Split(ps.Spec.Kind, "/")[0]
}

// Plugin derives a plugin name from the record
func (ps *specQuery) Plugin() Name {
	typeName := ""
	kind := strings.Split(ps.Spec.Kind, "/")
	if len(kind) > 1 {
		typeName = kind[1]
	}
	parts := strings.Split(ps.Spec.Metadata.Name, "/")
	if len(parts) > 1 {
		ps.instance = parts[1]
		if typeName != "" {
			return NameFrom(parts[0], typeName)
		}
		return NameFrom(parts[0], parts[1])
	}
	ps.instance = parts[0]

	if typeName != "" {
		return NameFrom(ps.Kind(), typeName)
	}
	return NameFrom(ps.Kind(), parts[0])
}

// Instance implements Addressable.Instance
func (ps specQuery) Instance() string {
	if ps.Spec.Metadata.Identity != nil {
		return ps.Spec.Metadata.Identity.ID
	}
	return ps.instance
}

// String returns the string represenation
func (ps specQuery) String() string {
	return fmt.Sprintf("%v::%v::%v", ps.Kind(), ps.Plugin(), ps.Instance())
}

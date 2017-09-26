package types

import (
	"time"

	"github.com/docker/infrakit/pkg/controller"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run/depends"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// DefaultSyncInterval is the default interval for syncing enrollments
	DefaultSyncInterval = 5 * time.Second
)

func init() {
	depends.Register("enroll", types.InterfaceSpec(controller.InterfaceSpec), ResolveDependencies)
}

// ResolveDependencies returns a list of dependencies by parsing the opaque Properties blob.
func ResolveDependencies(spec types.Spec) ([]plugin.Name, error) {
	if spec.Properties == nil {
		return nil, nil
	}

	properties := Properties{}
	err := spec.Properties.Decode(&properties)
	if err != nil {
		return nil, err
	}

	return []plugin.Name{properties.Instance.Plugin}, nil
}

// ListSourceUnion is a union type of possible values:
// a list of []intsance.Description
// a group plugin name
type ListSourceUnion types.Any

// InstanceDescriptions tries to 'cast' the union as list of descriptions
func (u *ListSourceUnion) InstanceDescriptions() ([]instance.Description, error) {
	list := []instance.Description{}
	err := (*types.Any)(u).Decode(&list)
	return list, err
}

// GroupPlugin tries to 'cast' the union value as a group plugin name
func (u *ListSourceUnion) GroupPlugin() (plugin.Name, error) {
	p := plugin.Name("")
	err := (*types.Any)(u).Decode(&p)
	return p, err
}

// UnmarshalJSON implements json.Unmarshaler
func (u *ListSourceUnion) UnmarshalJSON(buff []byte) error {
	*u = ListSourceUnion(*types.AnyBytes(buff))
	return nil
}

// MarshalJSON implements json.Marshaler
func (u *ListSourceUnion) MarshalJSON() ([]byte, error) {
	if u != nil {
		return (*types.Any)(u).MarshalJSON()
	}
	return []byte{}, nil
}

// PluginSpec has information about the plugin
type PluginSpec struct {
	// Plugin is the name of the instance plugin
	Plugin plugin.Name

	// Labels are the labels to use when querying for instances. This is the namespace.
	Labels map[string]string

	// Properties is the properties to configure the instance with.
	Properties *types.Any `json:",omitempty" yaml:",omitempty"`
}

// Properties is the schema of the configuration in the types.Spec.Properties
type Properties struct {

	// List is a list of instance descriptions to sync
	List *ListSourceUnion `json:",omitempty" yaml:",omitempty"`

	// Instance is the name of the instance plugin which will receive the
	// synchronization messages of provision / destroy based on the
	// changes in the List
	Instance PluginSpec
}

// Options is the controller options
type Options struct {

	// SourceKeySelector is a string template for selecting the join key from
	// a source instance.Description. This selector template should use escapes
	// so that the template {{ and }} are preserved.  For example,
	// SourceKeySelector: \{\{ .ID \}\}  # selects the ID field.
	SourceKeySelector string

	// SourceKeySelector is a string template for selecting the join key from
	// a enrollment plugin's instance.Description.
	EnrollmentKeySelector string

	// SyncInterval is the time interval between reconciliation. Syntax
	// is go's time.Duration string representation (e.g. 1m, 30s)
	SyncInterval types.Duration

	// DestroyOnTerminiate tells the controller to call instace.Destroy
	// for each member it is maintaining.  This is a matter of ownership
	// depending on use cases the controller may not *own* the data in the
	// downstream instance.  The controller merely reconciles it.
	DestroyOnTerminate bool
}

// TemplateFrom returns a template after it has un-escapes any escape sequences
func TemplateFrom(source []byte) (*template.Template, error) {
	buff := template.Unescape(source)
	return template.NewTemplate("str://"+string(buff), template.Options{MultiPass: false})
}

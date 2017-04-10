package types

// URL is a url string
type URL string

// Spec is the specification of the resource / object
type Spec struct {

	// Class is the kind/type of the resource -- e.g. instance-aws/ec2-instance
	Class string

	// SpiVersion is the name of the interface and version - instance/v0.1.0
	SpiVersion string

	// Metadata is metadata / useful information about object
	Metadata Metadata

	// Template is a template of the resource's properties
	Template URL `json:",omitempty" yaml:",omitempty"`

	// Properties is the desired properties of the resource, if template is specified,
	// then the values of properties override the same fields in template.
	Properties *Any

	// Options is additional data for handling the object that is not intrinsic to the object itself
	// but is necessary for the controllers working with it.
	Options *Any `json:",omitempty" yaml:",omitempty"`

	// Depends is a list of dependencies that this spec needs to have resolved before instances can
	// be instantiated.
	Depends []Dependency `json:",omitempty" yaml:",omitempty"`
}

// Validate checks the spec for validity
func (s Spec) Validate() error {
	if s.Class == "" {
		return errMissingAttribute("class")
	}
	if s.SpiVersion == "" {
		return errMissingAttribute("spiVersion")
	}
	if s.Metadata.Name == "" {
		return errMissingAttribute("metadata.name")
	}
	return nil
}

// Dependency models the reference and usage of another spec, by spec's Class and Name, and a way
// to extract its properties, and how it's referenced via the alias in the Properties section of the dependent Spec.
type Dependency struct {

	// Class is the Class of the spec this spec depends on
	Class string

	// Name is the Name of the spec this spec dependes on
	Name string

	// Selector is a query expression in jmespath that can select some value to be bound to the alias
	Selector string

	// Var is the name givent to reference the selected value. This is then referenced in the {{ var }} expressions
	// inside the Template or Properties fields of the spec.
	Var string
}

// Identity uniquely identifies an instance
type Identity struct {

	// UID is a unique identifier for the object instance.
	UID string
}

// Metadata captures label and descriptive information about the object
type Metadata struct {

	// Identity is an optional component that exists only in the case of a real object instance.
	*Identity `json:",omitempty" yaml:",omitempty"`

	// Name is a user-friendly name.  It may or may not be unique.
	Name string

	// Tags are a collection of labels, in key-value form, about the object
	Tags map[string]string
}

// Object is an instance or realization of the Spec.  It has Spec as the desired attributes, as well as,
// the instance identifier (ID), and State, which represents the current snapshot of the object instance.
type Object struct {

	// Spec is the specification / desired state of the object instance.
	Spec

	// State is the current snapshot / status of the object instance.
	State *Any `json:",omitempty" yaml:",omitempty"`
}

// Validate checks the object for validity
func (o Object) Validate() error {
	err := o.Spec.Validate()
	if err != nil {
		return err
	}

	if o.Metadata.Identity.UID == "" {
		return errMissingAttribute("metadata.identity.uid")
	}
	return nil
}

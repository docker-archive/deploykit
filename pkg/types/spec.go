package types

import (
	"fmt"
	"net/url"
	"strings"
)

// Spec is the specification of the resource / object
type Spec struct {

	// Kind is the category of the resources and kind can have types  -- e.g. instance-aws/ec2-instance
	Kind string `json:"kind"`

	// Version is the name of the interface and version - instance/v0.1.0
	Version string `json:"version"`

	// Metadata is metadata / useful information about object
	Metadata Metadata `json:"metadata"`

	// Template is a template of the resource's properties
	Template *URL `json:"template,omitempty" yaml:",omitempty"`

	// Properties is the desired properties of the resource, if template is specified,
	// then the values of properties override the same fields in template.
	Properties *Any `json:"properties"`

	// Options is additional data for handling the object that is not intrinsic to the object itself
	// but is necessary for the controllers working with it.
	Options *Any `json:"options,omitempty" yaml:",omitempty"`

	// Depends is a list of dependencies that this spec needs to have resolved before instances can
	// be instantiated.
	Depends []Dependency `json:"depends,omitempty" yaml:",omitempty"`
}

// Validate checks the spec for validity
func (s Spec) Validate() error {
	if s.Kind == "" {
		return errMissingAttribute("kind")
	}
	if s.Version == "" {
		return errMissingAttribute("version")
	}
	if s.Metadata.Name == "" {
		return errMissingAttribute("metadata.name")
	}
	return nil
}

// Compare implements the comparable. This implementation will compute a finger print and use that
// as a comparison.  So
func (s Spec) Compare(other Spec) int {
	if s.Kind < other.Kind {
		return -1
	}
	if s.Kind > other.Kind {
		return 1
	}
	if s.Version < other.Version {
		return -1
	}
	if s.Version > other.Version {
		return 1
	}
	if c := s.Metadata.Compare(other.Metadata); c != 0 {
		return c
	}
	if s.Template != nil && other.Template != nil {
		if c := s.Template.Compare(*other.Template); c != 0 {
			return c
		}
	}
	// By now we can't disambiguate enough... so just compute fingerprints of the whole thing
	a, err := AnyValue(s)
	if err != nil {
		return -1
	}
	b, err := AnyValue(other)
	if err != nil {
		return 1
	}
	f1 := Fingerprint(a)
	f2 := Fingerprint(b)
	if f1 < f2 {
		return -1
	}
	if f1 > f2 {
		return 1
	}
	return 0
}

// Dependency models the reference and usage of another spec, by spec's Kind and Name, and a way
// to extract its properties, and how it's referenced via the alias in the Properties section of the dependent Spec.
type Dependency struct {

	// Kind is the Kind of the spec this spec depends on
	Kind string `json:"kind"`

	// Name is the Name of the spec this spec dependes on
	Name string `json:"name"`

	// Bind is an associative array of pointer to the fields in the object to a variable name that will be referenced
	// in the properties or template of the owning spec.
	Bind map[string]*Pointer `json:"bind"`
}

// Identity uniquely identifies an instance
type Identity struct {
	// ID is a unique identifier for the object instance.
	ID string `json:"id" yaml:"id"`
}

// Compare implments comparable
func (i Identity) Compare(other Identity) int {
	switch {
	case i.ID == other.ID:
		return 0
	case i.ID < other.ID:
		return -1
	}
	return 1
}

// Metadata captures label and descriptive information about the object
type Metadata struct {

	// Identity is an optional component that exists only in the case of a real object instance.
	*Identity `json:",inline,omitempty" yaml:",inline,omitempty"`

	// Name is a user-friendly name.  It may or may not be unique.
	Name string `json:"name"`

	// Tags are a collection of labels, in key-value form, about the object
	Tags map[string]string `json:"tags"`
}

// Fingerprint returns a unqiue key based on the content of this
func (m Metadata) Fingerprint() string {
	return Fingerprint(AnyValueMust(m))
}

// Compare implements comparable
func (m Metadata) Compare(other Metadata) int {
	if m.Identity != nil && other.Identity != nil {
		return m.Identity.Compare(*other.Identity)
	}
	switch {
	case m.Name < other.Name:
		return -1
	case m.Name > other.Name:
		return 1
	}
	// when names are same compare the tags
	if len(m.Tags) < len(other.Tags) {
		return -1
	}
	if len(m.Tags) > len(other.Tags) {
		return 1
	}
	for k, v := range m.Tags {
		if v == other.Tags[k] {
			continue
		}
		if v < other.Tags[k] {
			return -1
		}
		return 1
	}
	return 0
}

// AddTagsFromStringSlice will parse any '=' delimited strings and set the Tags map. It overwrites on duplicate keys.
func (m Metadata) AddTagsFromStringSlice(v []string) Metadata {
	other := m
	if other.Tags == nil {
		other.Tags = map[string]string{}
	}
	for _, vv := range v {
		p := strings.Split(vv, "=")
		other.Tags[p[0]] = p[1]
	}
	return other
}

// URL is an alias of url
type URL url.URL

// NewURL creates a new url from string
func NewURL(s string) (*URL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	v := URL(*u)
	return &v, nil
}

// Compare implements comparable
func (u URL) Compare(other URL) int {
	if u.String() < other.String() {
		return -1
	}
	if u.String() > other.String() {
		return 1
	}
	return 0
}

// Absolute returns true if the url is absolute (not relative)
func (u URL) Absolute() bool {
	return url.URL(u).Scheme != ""
}

// Value returns the aliased struct
func (u URL) Value() *url.URL {
	copy := url.URL(u)
	return &copy
}

// String returns the string representation of the URL
func (u URL) String() string {
	return u.Value().String()
}

// MarshalJSON returns the json representation
func (u URL) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, u.String())), nil
}

// UnmarshalJSON unmarshals the buffer to this struct
func (u *URL) UnmarshalJSON(buff []byte) error {
	str := strings.Trim(string(buff), " \"\\'\t\n")
	uu, err := url.Parse(str)
	if err != nil {
		return err
	}
	copy := URL(*uu)
	*u = copy
	return nil
}

// MustSpec returns the spec or panic if errors
func MustSpec(s Spec, err error) Spec {
	if err != nil {
		panic(err)
	}
	return s
}

// SpecFromString returns the Specs from input string as YAML or JSON
func SpecFromString(s string) (Spec, error) {
	return SpecFromBytes([]byte(s))
}

// SpecFromBytes parses the input either as YAML or JSON and returns the Specs
func SpecFromBytes(b []byte) (Spec, error) {
	out := Spec{}
	any, err := AnyYAML(b)
	if err != nil {
		any = AnyBytes(b)
	}
	err = any.Decode(&out)
	return out, err
}

package types

// Object is an instance or realization of the Spec.  It has Spec as the desired attributes, as well as,
// the instance identifier (ID), and State, which represents the current snapshot of the object instance.
type Object struct {

	// Spec is the specification / desired state of the object instance.
	Spec

	// State is the current snapshot / status of the object instance.
	State *Any `json:"state,omitempty" yaml:",omitempty"`
}

// Validate checks the object for validity
func (o Object) Validate() error {
	err := o.Spec.Validate()
	if err != nil {
		return err
	}
	if o.Metadata.Identity == nil {
		return errMissingAttribute("metadata.identity")
	}
	if o.Metadata.Identity.UID == "" {
		return errMissingAttribute("metadata.identity.uid")
	}
	return nil
}

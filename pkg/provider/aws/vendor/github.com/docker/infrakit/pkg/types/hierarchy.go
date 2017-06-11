package types

// Hierarchical is the interface for any hierarchical, tree-like structures
type Hierarchical interface {

	// List returns a list of *child nodes* given a path, which is specified as a slice
	List(path Path) (child []string, err error)

	// Get retrieves the value at path given.
	Get(path Path) (value *Any, err error)
}

type hierarchical map[string]interface{}

// HierarchicalFromMap adapts a map to the hierarchical interface
func HierarchicalFromMap(m map[string]interface{}) Hierarchical {
	return hierarchical(m)
}

// List lists the children under a path
func (h hierarchical) List(path Path) (child []string, err error) {
	return List([]string(path), h), nil
}

// Get gets the value at path
func (h hierarchical) Get(path Path) (value *Any, err error) {
	v := Get([]string(path), h)
	return AnyValue(v)
}

// ListAll returns all the paths under the start path, unsorted
func ListAll(h Hierarchical, start Path) ([]Path, error) {
	all := []Path{}
	children, err := h.List(start)
	if err != nil {
		return all, err
	}

	for _, c := range children {
		p := start.JoinString(c)
		all = append(all, p)

		subs, err := ListAll(h, p)
		if err != nil {
			return all, err
		}
		all = append(all, subs...)
	}
	return all, nil
}

package metadata

// Path is used to identify a particle of metadata.  The path can be strings separated by / as in a URL.
type Path []string

// Len returns the length of the path
func (p Path) Len() int {
	return len([]string(p))
}

// Index returns the ith component in the path
func (p Path) Index(i int) *string {
	if p.Len() <= i {
		return nil
	}
	copy := []string(p)[i]
	return &copy
}

// Shift returns a new path that's shifted i positions to the left -- ith child of the head at index=0
func (p Path) Shift(i int) Path {
	len := p.Len() - i
	if len <= 0 {
		return Path([]string{})
	}
	new := make([]string, len)
	copy(new, []string(p)[i:])
	return Path(new)
}

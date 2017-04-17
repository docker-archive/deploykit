package types

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

var (
	// NullPath means no path
	NullPath = Path([]string{})
)

// RFC6901ToPath takes a path expression in the format of IETF RFC6901 (JSON pointer) and convert it to a Path
func RFC6901ToPath(path string) Path {
	return rfc6901ToPath(strings.Split(path, "/"))
}

func rfc6901ToPath(path []string) Path {
	decoded := []string{}
	for _, p := range path {
		p = strings.Replace(p, `~1`, `/`, -1)
		p = strings.Replace(p, `~0`, `~`, -1)
		if _, err := strconv.Atoi(p); err == nil {
			p = fmt.Sprintf(`[%s]`, p)
		}
		decoded = append(decoded, p)
	}
	return Path(decoded)
}

// Path is used to identify a particle of metadata.  The path can be strings separated by / as in a URL.
type Path []string

// PathFromString returns the path components of a / separated path
func PathFromString(path string) Path {
	if path == "" {
		return Path([]string{}).Clean() // ==> .
	}
	return Path(strings.Split(path, "/")).Clean()
}

// PathFrom return a single path of the given components
func PathFrom(a string, b ...string) Path {
	p := Path(append([]string{a}, b...))
	return p.Clean()
}

// PathFromStrings returns the path from a list of strings
func PathFromStrings(a string, b ...string) []Path {
	list := []Path{PathFromString(a)}
	for _, p := range b {
		list = append(list, PathFromString(p))
	}
	return list
}

// String returns the string representation of path
func (p Path) String() string {
	s := strings.Join([]string(p), "/")
	if len(s) == 0 {
		return "."
	}
	return s
}

// Valid returns true if is a valid path
func (p Path) Valid() bool {
	return p.Len() > 0
}

// Dot returns true if this is a .
func (p Path) Dot() bool {
	return len(p) == 1 && p[0] == "."
}

// Clean scrubs the path to remove any empty string or . or .. and collapse the path into a concise form.
// It's similar to path.Clean in the standard lib.
func (p Path) Clean() Path {
	this := []string(p)
	copy := []string{}
	for i, v := range this {
		switch v {
		case ".":
		case "":
			if i == 0 {
				copy = append(copy, "")
			}
		case "..":
			if len(copy) == 0 {
				copy = append(copy, "..")
			} else {
				copy = copy[0 : len(copy)-1]
				if len(copy) == 0 {
					return NullPath
				}
			}
		default:
			copy = append(copy, v)
		}
	}
	if len(copy) == 0 {
		copy = []string{"."}
	} else if this[len(this)-1] == "" || this[len(this)-1] == "." {
		if len(this)-2 > -1 {
			copy = append(copy, "")
		}
	}

	return Path(copy)
}

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

// Dir returns the 'dir' of the path
func (p Path) Dir() Path {
	pp := p.Clean()
	if len(pp) > 1 {
		return p[0 : len(pp)-1]
	}
	return Path([]string{"."})
}

// Base returns the base of the path
func (p Path) Base() string {
	pp := p.Clean()
	return pp[len(pp)-1]
}

// JoinString joins the input as a child of this path
func (p Path) JoinString(child string) Path {
	return p.Join(Path([]string{child}))
}

// Join joins the child to the parent
func (p Path) Join(child Path) Path {
	pp := p.Clean()
	this := []string(pp)
	if this[len(this)-1] == "" {
		pp = Path(this[:len(this)-1])
	}
	return Path(append(pp, []string(child)...)).Clean()
}

// Rel returns a new path that is a child of the input from this path.
// e.g. For a path a/b/c/d Rel(a/b/) returns c/d.  NullPath is returned if
// the two are not relative to one another.
func (p Path) Rel(path Path) Path {
	if path.Equal(PathFromString(".")) {
		return p
	}

	this := []string(p.Clean())
	parent := []string(path.Clean())
	if parent[len(parent)-1] == "" {
		parent = parent[:len(parent)-1]
	}
	if len(this) < len(parent) {
		return NullPath
	}
	for i := 0; i < len(parent); i++ {
		if parent[i] != this[i] {
			return NullPath
		}
	}
	return Path(this[len(parent):])
}

// Equal returns true if the path is lexicographically equal to the other
func (p Path) Equal(other Path) bool {
	if len(p) != len(other) {
		return false
	}
	for i := 0; i < len(p); i++ {
		if p[i] != other[i] {
			return false
		}
	}
	return true
}

// Less return true if the path is lexicographically less than the other
func (p Path) Less(other Path) bool {
	min := len(p)
	if len(other) < min {
		min = len(other)
	}
	for i := 0; i < min; i++ {
		if string(p[i]) != string(other[i]) {
			return string(p[i]) < string(other[i])
		}
	}
	return len(p) < len(other)
}

type pathSorter []Path

func (p pathSorter) Len() int           { return len(p) }
func (p pathSorter) Less(i, j int) bool { return Path(p[i]).Less(Path(p[j])) }
func (p pathSorter) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Sort sorts the paths
func Sort(p []Path) {
	sort.Sort(pathSorter(p))
}

// MarshalJSON returns the json representation
func (p Path) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, p.String())), nil
}

// UnmarshalJSON unmarshals the buffer to this struct
func (p *Path) UnmarshalJSON(buff []byte) error {
	str := strings.Trim(string(buff), " \"\\'\t\n")
	*p = PathFromString(str)
	return nil
}

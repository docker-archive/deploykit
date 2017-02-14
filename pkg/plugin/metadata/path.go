package metadata

import (
	"strings"

	"github.com/docker/infrakit/pkg/spi/metadata"
)

// Path returns the path compoments of a / separated path
func Path(path string) metadata.Path {
	return metadata.Path(strings.Split(path, "/"))
}

// PathFromStrings returns the path from a list of strings
func PathFromStrings(a string, b ...string) metadata.Path {
	if a != "" {
		return metadata.Path(append([]string{a}, b...))
	}
	return metadata.Path(b)
}

// String returns the string representation of path
func String(p metadata.Path) string {
	s := strings.Join([]string(p), "/")
	if strings.Index(s, "/") == -1 {
		return "/" + s
	}
	return s
}

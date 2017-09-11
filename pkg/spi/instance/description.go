package instance

import (
	"github.com/docker/infrakit/pkg/types"
)

// Fingerprint returns the fingerprint of the spec
func (d Description) Fingerprint() string {
	return types.Fingerprint(types.AnyValueMust(d))
}

// Compare compares the two descriptions by ID
func (d Description) Compare(other Description) int {
	if d.ID < other.ID {
		return -1
	}
	if d.ID > other.ID {
		return 1
	}
	return 0
}

// Descriptions is a collection of descriptions
type Descriptions []Description

// Len is part of sort.Interface.
func (list Descriptions) Len() int {
	return len(list)
}

// Swap is part of sort.Interface.
func (list Descriptions) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (list Descriptions) Less(i, j int) bool {
	return list[i].Compare(list[j]) < 0
}

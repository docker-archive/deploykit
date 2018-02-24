package instance

import (
	"sort"

	"github.com/deckarep/golang-set"
	"github.com/docker/infrakit/pkg/template"
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

// View returns a view of the Description given the text template. The text template
// can contain escaped \{\{\}\} template expression delimiters.
func (d Description) View(viewTemplate string) (string, error) {
	buff := template.Unescape([]byte(viewTemplate))
	t, err := template.NewTemplate(
		"str://"+string(buff),
		template.Options{MultiPass: false, MissingKey: template.MissingKeyError},
	)
	if err != nil {
		return "", err
	}
	// this is a substitute struct type to make the Properties searchable
	type desc struct {
		ID         ID
		LogicalID  *LogicalID
		Tags       map[string]string
		Properties interface{}
	}

	var p interface{}
	if err := d.Properties.Decode(&p); err != nil {
		return "", err
	}

	v := desc{
		ID:         d.ID,
		LogicalID:  d.LogicalID,
		Tags:       d.Tags,
		Properties: p,
	}
	return t.Render(v)
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

// KeyFunc is a function that extracts the key from the description
type KeyFunc func(Description) (string, error)

// DescriptionIndex is a struct containing the keys and values
type DescriptionIndex struct {
	Keys mapset.Set
	Map  map[string]Description
}

// Select returns a slice of Descriptions matching the keys in the given set. Output is sorted
func (i *DescriptionIndex) Select(keys mapset.Set) Descriptions {
	out := Descriptions{}
	for n := range keys.Iter() {
		out = append(out, i.Map[n.(string)])
	}
	sort.Sort(out)
	return out
}

// Descriptions returns a slice of Descriptions. Output is sorted
func (i *DescriptionIndex) Descriptions() Descriptions {
	return i.Select(i.Keys)
}

// Index indexes the descriptions
func (list Descriptions) Index(getKey KeyFunc) (*DescriptionIndex, error) {
	// Track errors and return what could be indexed
	var e error
	index := map[string]Description{}
	this := mapset.NewSet()
	for _, n := range list {
		key, err := getKey(n)
		if err != nil {
			e = err
			continue
		}
		this.Add(key)
		index[key] = n
	}
	return &DescriptionIndex{
		Map:  index,
		Keys: this,
	}, e
}

// Difference returns a list of specs that is not in the receiver.
func Difference(list Descriptions, listKeyFunc KeyFunc,
	other Descriptions, otherKeyFunc KeyFunc) Descriptions {

	thisIndex, _ := list.Index(listKeyFunc)
	thatIndex, _ := other.Index(otherKeyFunc)

	return thisIndex.Select(thisIndex.Keys.Difference(thatIndex.Keys))
}

package core

import (
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "core")

// Constructor is a function that can construct an instance using the spec
type Constructor func(*Spec, Objects) (*Object, error)

// SpecsFromURL loads the raw specs from the URL and returns the root url and raw bytes
func SpecsFromURL(uri string) (root string, config []byte, err error) {
	buff, err := template.Fetch(uri, template.Options{})
	if err != nil {
		return uri, nil, err
	}

	// try to decode it as json...
	var v interface{}
	err = types.AnyBytes(buff).Decode(&v)
	if err == nil {
		return uri, buff, nil
	}

	y, err := types.AnyYAML(buff)
	if err != nil {
		return uri, buff, err
	}
	err = y.Decode(&v)
	if err != nil {
		return uri, nil, err
	}
	return uri, y.Bytes(), nil
}

// NormalizeSpecs given the input bytes and its source, returns the normalized specs where
// template urls have been updated to be absolute and the specs are in dependency order.
func NormalizeSpecs(uri string, input []byte) ([]*types.Spec, error) {
	parsed := []*types.Spec{}
	if err := types.AnyBytes(input).Decode(&parsed); err != nil {
		return nil, err
	}

	specs := []*types.Spec{}

	for _, member := range parsed {
		specs = append(specs, member)
		specs = append(specs, Nested(member)...)
	}

	// compute ordering
	ordered, err := OrderByDependency(specs)
	if err != nil {
		return nil, err
	}

	log.Debug("ordered by dependency", "count", len(ordered), "unordered", len(specs))

	// normalize all the template references with respect to the source url
	for _, spec := range ordered {

		if spec.Template != nil && !spec.Template.Absolute() {
			absolute, err := template.GetURL(uri, spec.Template.String())
			if err != nil {
				return nil, err
			}
			if u, err := types.NewURL(absolute); err == nil {
				spec.Template = u
			}
		}
	}

	return ordered, nil
}

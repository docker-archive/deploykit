package core

import (
	"encoding/json"
	"reflect"

	"github.com/docker/infrakit/pkg/types"
	"github.com/twmb/algoimpl/go/graph"
)

// converts a map to a Spec, nil if it cannot be done
func mapToSpec(m map[string]interface{}) *types.Spec {
	// This is hacky -- generate a string representation
	// and try to parse it as struct
	buff, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	s := types.Spec{}
	err = json.Unmarshal(buff, &s)
	if err != nil {
		return nil
	}
	if s.Validate() == nil {
		return &s
	}
	return nil
}

// findSpecs parses the bytes and returns a Spec, if the Spec can be parsed
// from the buffer.  Some fields are verified and must be present for the
// buffer to be considered a representation of a Spec.
func findSpecs(v interface{}) []*types.Spec {

	result := []*types.Spec{}

	switch v := v.(type) {

	case []*types.Spec:
		for _, x := range v {
			c := *x
			result = append(result, findSpecs(&c)...)
		}

	case []types.Spec:
		for _, x := range v {
			c := x
			result = append(result, findSpecs(&c)...)
		}

	case []interface{}:
		for _, x := range v {
			c := x
			result = append(result, findSpecs(c)...)
		}

	case map[string]interface{}:
		// convert to Spec?
		result = append(result, findSpecs(mapToSpec(v))...)

	case *types.Any:

		if v == nil {
			return result
		}

		spec := types.Spec{}
		if err := v.Decode(&spec); err == nil {

			if spec.Validate() == nil {
				result = append(result, findSpecs(&spec)...)
				return result
			}

		}

		// now decode as regular struct - map or []interface{}
		var vv interface{}
		if err := v.Decode(&vv); err != nil {
			return nil
		}

		switch vv := vv.(type) {
		case []interface{}:
			for _, x := range vv {
				result = append(result, findSpecs(x)...)
			}
		case map[string]interface{}:
			for _, x := range vv {
				result = append(result, findSpecs(x)...)
			}
		}

	case types.Spec:
		c := v
		result = append(result, &c)
		result = append(result, findSpecs(c.Properties)...)

	case *types.Spec:

		if v == nil {
			return result
		}

		c := *v
		result = append(result, &c)
		result = append(result, findSpecs(c.Properties)...)

	default:
		value := reflect.Indirect(reflect.ValueOf(v))
		if value.Type().Kind() == reflect.Struct {
			for i := 0; i < value.NumField(); i++ {
				fv := value.Field(i)
				if fv.IsValid() {
					result = append(result, findSpecs(fv.Interface())...)
				}
			}
		}
	}
	return result
}

// nested recurses through the Properties of the spec and returns any nested specs.
func nested(s *types.Spec) []*types.Spec {
	if s.Properties == nil {
		return nil
	}
	return findSpecs(s.Properties)
}

type key struct {
	kind string
	name string
}

func indexSpecs(specs []*types.Spec, g *graph.Graph) map[key]*graph.Node {
	index := map[key]*graph.Node{}
	for _, spec := range specs {

		node := g.MakeNode()
		*(node.Value) = spec

		index[key{kind: spec.Kind, name: spec.Metadata.Name}] = &node
	}
	return index
}

func indexGet(index map[key]*graph.Node, kind, name string) *graph.Node {
	if v, has := index[key{kind: kind, name: name}]; has {
		return v
	}
	return nil
}

// AllSpecs returns a list of all the specs given plus any nested specs
func AllSpecs(specs []*types.Spec) []*types.Spec {
	all := []*types.Spec{}
	for _, s := range specs {
		all = append(all, s)
		all = append(all, nested(s)...)
	}
	return all
}

// OrderByDependency returns the given specs in dependency order.  The input is assume to be exhaustive in that
// all nested specs are part of the list.
func OrderByDependency(specs []*types.Spec) ([]*types.Spec, error) {

	g := graph.New(graph.Directed)
	if g == nil {
		return nil, nil
	}

	index := indexSpecs(specs, g)

	for _, spec := range specs {

		from := indexGet(index, spec.Kind, spec.Metadata.Name)
		if from == nil {
			return nil, errNotFound{kind: spec.Kind, name: spec.Metadata.Name}
		}

		for _, depend := range spec.Depends {

			to := indexGet(index, depend.Kind, depend.Name)
			if to == nil {
				return nil, errBadDependency(depend)
			}

			if from == to {

				a := (*from.Value).(*types.Spec)
				b := (*to.Value).(*types.Spec)
				return nil, errCircularDependency([]*types.Spec{a, b})
			}

			if err := g.MakeEdge(*to, *from); err != nil {
				return nil, err
			}
		}
	}

	// cycle detection
	for _, connected := range g.StronglyConnectedComponents() {
		if len(connected) > 1 {

			cycle := []*types.Spec{}
			for _, n := range connected {
				cycle = append(cycle, (*n.Value).(*types.Spec))
			}
			return nil, errCircularDependency(cycle)
		}
	}

	ordered := []*types.Spec{}
	for _, n := range g.TopologicalSort() {
		ordered = append(ordered, (*n.Value).(*types.Spec))
	}

	return ordered, nil
}

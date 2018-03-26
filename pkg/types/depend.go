package types

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/twmb/algoimpl/go/graph"
)

const dependRegexStr = "\\@depend\\('(([[:alnum:]]|-|_|:|\\.|/|\\[|\\])+)'\\)\\@"

// dependRegex is the regex for the special format of a string value to denote a dependency
// on another resource's property field.  Eg. "@depends('net1/cidr')@"
var dependRegex = regexp.MustCompile(dependRegexStr)

// Depend is a specification of a dependency
type Depend string

// NewDepend returns a Depend for a single expression
func NewDepend(p string) Depend {
	return Depend(fmt.Sprintf("\"@depend('%s')@\"", p))
}

// Parse parses the expression into a path.  If it's not a valid expression, false is returned.
// TODO - this assumes there's only 1 expression and it's the entire string. This won't work
// for cases like an Init script where a script like `cluster join --token @depend('a')@ --flag @depend('b')@ 10.1.1.1`
func (d Depend) Parse() (found []Path, hasMatches bool) {
	for _, m := range dependRegex.FindAllStringSubmatch(string(d), -1) {
		found = append(found, PathFromString(m[1]))
	}
	hasMatches = len(found) > 0
	return
}

// EvalDepends takes a value that possibly have a number of depend expressions and evaluate
// all expression within and substitute values from the fetcher.
func EvalDepends(v interface{}, fetcher func(Path) (interface{}, error)) interface{} {
	switch v := v.(type) {
	case *Any:
		var f interface{}
		if err := v.Decode(&f); err == nil {
			return EvalDepends(f, fetcher)
		}
	case map[string]interface{}:
		for k, vv := range v {
			v[k] = EvalDepends(vv, fetcher)
		}
	case []interface{}:
		for i, vv := range v {
			v[i] = EvalDepends(vv, fetcher)
		}
	case string:
		if found, ok := Depend(v).Parse(); ok {

			// If there are more than one '@depend@' expression, then we'd assume the field
			// value is a string, because it doesn't make any sense to "concatenate" two ints or floats.
			if len(found) == 1 {
				// found a depend, now get the real value and swap
				substituted, err := fetcher(found[0])
				if err != nil {
					substituted = err.Error()
				}
				if substituted == nil {
					// no value found, just return original expression
					return fmt.Sprintf("@depend('%s')@", found[0].String())
				}
				if _, is := substituted.(string); !is {
					return substituted // for a string expression that evals to a non-string type
				}
			}

			// Otherwise we assume this is going to be text with 0 or more expressions
			text := v
			for _, p := range found {

				// Pretty hacky: since all the paths are tokenized in order as they appear
				// we simply reconstruct the original expressions and use them as separators to split the text
				// and perform substitutions from left to right until the end.
				separator := fmt.Sprintf("@depend('%s')@", p.String())

				// found a depend, now get the real value and swap
				substituted, err := fetcher(p)
				if err != nil {
					substituted = err.Error()
				}
				if substituted == nil {
					substituted = separator // if we don't have value, then use the original expression
				}

				parts := strings.Split(text, separator)
				text = strings.Join(append([]string{parts[0], fmt.Sprintf("%v", substituted)}, parts[1:]...), "")
			}
			return text
		}
	default:
	}
	return v
}

// ParseDepends parses the blob and returns a list of paths. The path's first component is the
// name of the resource. e.g. dep `net1/cidr`
func ParseDepends(any *Any) []Path {
	var v interface{}
	err := any.Decode(&v)
	if err != nil {
		return nil
	}
	l := parse(v, []Path{})
	SortPaths(l)
	return l
}

func parse(v interface{}, found []Path) (out []Path) {
	switch v := v.(type) {
	case map[string]interface{}:
		for _, vv := range v {
			out = append(out, parse(vv, nil)...)
		}
	case []interface{}:
		for _, vv := range v {
			out = append(out, parse(vv, nil)...)
		}
	case string:
		if found, ok := Depend(v).Parse(); ok {
			for _, p := range found {
				out = append(out, p)
			}
		}
	default:
	}
	out = append(found, out...)
	return
}

// converts a map to a Spec, nil if it cannot be done
func mapToSpec(m map[string]interface{}) *Spec {
	// This is hacky -- generate a string representation
	// and try to parse it as struct
	buff, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	s := Spec{}
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
func findSpecs(v interface{}) []*Spec {

	result := []*Spec{}

	switch v := v.(type) {

	case []*Spec:
		for _, x := range v {
			c := *x
			result = append(result, findSpecs(&c)...)
		}

	case []Spec:
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

	case *Any:

		if v == nil {
			return result
		}

		spec := Spec{}
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

	case Spec:
		c := v
		result = append(result, &c)
		result = append(result, findSpecs(c.Properties)...)

	case *Spec:

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

// Flatten recurses through the Properties of the spec and returns any nested specs.
func Flatten(s *Spec) []*Spec {
	if s.Properties == nil {
		return nil
	}
	return findSpecs(s.Properties)
}

type specKey struct {
	kind string
	name string
}

func indexSpecs(specs []*Spec, g *graph.Graph) map[specKey]*graph.Node {
	index := map[specKey]*graph.Node{}
	for _, spec := range specs {

		node := g.MakeNode()
		*(node.Value) = spec

		index[specKey{kind: spec.Kind, name: spec.Metadata.Name}] = &node
	}
	return index
}

func indexGet(index map[specKey]*graph.Node, kind, name string) *graph.Node {
	if v, has := index[specKey{kind: kind, name: name}]; has {
		return v
	}
	return nil
}

// AllSpecs returns a list of all the specs given plus any nested specs
func AllSpecs(specs []*Spec) []*Spec {
	all := []*Spec{}
	for _, s := range specs {
		all = append(all, s)
		all = append(all, Flatten(s)...)
	}
	return all
}

// OrderByDependency returns the given specs in dependency order.  The input is assume to be exhaustive in that
// all nested specs are part of the list.
func OrderByDependency(specs []*Spec) ([]*Spec, error) {

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

				a := (*from.Value).(*Spec)
				b := (*to.Value).(*Spec)
				return nil, errCircularDependency([]*Spec{a, b})
			}

			if err := g.MakeEdge(*to, *from); err != nil {
				return nil, err
			}
		}
	}

	// cycle detection
	for _, connected := range g.StronglyConnectedComponents() {
		if len(connected) > 1 {

			cycle := []*Spec{}
			for _, n := range connected {
				cycle = append(cycle, (*n.Value).(*Spec))
			}
			return nil, errCircularDependency(cycle)
		}
	}

	ordered := []*Spec{}
	for _, n := range g.TopologicalSort() {
		ordered = append(ordered, (*n.Value).(*Spec))
	}

	return ordered, nil
}

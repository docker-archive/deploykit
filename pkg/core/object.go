package core

import (
	"fmt"
	"strings"
	"sync"

	"github.com/docker/infrakit/pkg/fsm"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

// Objects is a collection of instances instantiated from specs.  They are queryable via the FindBy method
type Objects interface {
	FindBy(...interface{}) *types.Object
	Add(*types.Object)
	Remove(*types.Object)
	Len() int
}

type objectIndex struct {
	objects map[interface{}]*types.Object
	lock    sync.Mutex
	keyFunc func(*types.Object) interface{}
}

// NewObjects creates an index
func NewObjects(key func(*types.Object) []interface{}) Objects {
	return &objectIndex{
		objects: map[interface{}]*types.Object{},
		keyFunc: func(o *types.Object) interface{} {
			return fmt.Sprintf("%v", key(o))
		},
	}
}

// FindBy returns an object matching the args
func (index *objectIndex) FindBy(args ...interface{}) *types.Object {
	index.lock.Lock()
	defer index.lock.Unlock()

	key := fmt.Sprintf("%v", args)
	return index.objects[key]
}

// Add adds an object
func (index *objectIndex) Add(o *types.Object) {
	index.lock.Lock()
	defer index.lock.Unlock()

	key := index.keyFunc(o)
	index.objects[key] = o
}

// Remove removes an object
func (index *objectIndex) Remove(o *types.Object) {
	index.lock.Lock()
	defer index.lock.Unlock()

	key := index.keyFunc(o)
	delete(index.objects, key)
}

// Len returns the size
func (index *objectIndex) Len() int {
	return len(index.objects)
}

// resolveDepends resolves the dependency declared in the Depends section of the Object by querying
// the index of objects and applying the json pointer to get the values.  Then the result as a map
// of key/value is returned.  The result can then be used as a template context.
func resolveDepends(o *types.Object, objects Objects) (depends map[string]interface{}, err error) {

	depends = map[string]interface{}{}

	for _, dep := range o.Depends {

		otherObject := objects.FindBy(dep.Kind, dep.Name)
		if otherObject == nil {
			err = fmt.Errorf("unresolved dependency: %v", dep)
			return
		}

		// Here we encode the entire other object to a map. This is because the expressions
		// used to reference values (json pointers) follow the structure of the document and not the
		// golang structs which can be different due to embedding of fields and lower-case fields.
		other := map[string]interface{}{}

		any, e := types.AnyValue(otherObject)
		if e != nil {
			err = e
			return
		}
		if e := any.Decode(&other); e != nil {
			err = e
			return
		}

		for key, pointer := range dep.Bind {
			depends[key] = pointer.Get(other)
		}
	}
	return
}

func templateEngine(url string,
	object *types.Object,
	depends map[string]interface{},
	scope scope.Scope) (*template.Template, error) {

	t, err := scope.TemplateEngine(url, template.Options{})
	if err != nil {
		return nil, err
	}

	// add the key / value pairs in depends as vars
	for k, v := range depends {
		t.Global(k, v)
	}

	// convert object as a []interface{} or map[string]interface{}
	// so that the case in the var expressions will match the casing of fields in the doc. e.g. `tags` instead of `Tags`
	var objectView interface{}
	if any, err := types.AnyValue(object); err != nil {
		return nil, err
	} else if err := any.Decode(&objectView); err != nil {
		return nil, err
	}

	t.WithFunctions(func() []template.Function {
		return []template.Function{
			{
				Name: "var",
				Description: []string{
					"Metadata function takes a path of the form \"plugin_name/path/to/data\"",
					"and calls GET on the plugin with the path \"path/to/data\".",
					"It's identical to the CLI command infrakit metadata cat ...",
				},
				Func: func(n string, optional ...interface{}) (interface{}, error) {

					// It's chained -- first we try to get value from the object itself
					// then we try the default template var
					p := types.PathFromString(n)

					v := types.Get(p, objectView)
					if v != nil {
						return v, nil
					}
					return t.Var(n, optional...)
				},
			},
		}
	})

	return t, nil
}

// renderProperties applies the template and the properties to produce the final properties
func renderProperties(object *types.Object, id fsm.ID,
	depends map[string]interface{},
	scope scope.Scope) (*types.Any, error) {

	var properties interface{} // this will be serialized into any

	evaluatedTemplate := false

	// If the spec has a Template and Properties, then render the template
	if object.Spec.Template != nil {

		t, err := templateEngine(object.Spec.Template.String(), object, depends, scope)
		if err != nil {
			return nil, err
		}

		view, err := t.Render(id)
		if err != nil {
			return nil, err
		}

		jsonBuff, err := ensureJSON([]byte(view))
		if err != nil {
			return nil, err
		}

		if err := types.AnyBytes(jsonBuff).Decode(&properties); err != nil {
			return nil, err
		}

		evaluatedTemplate = true
	}

	// We care about the properties only if we haven't evaluated the template. If the template
	// was present, the values in the Properties would have been taking into account.

	if object.Spec.Properties != nil && !evaluatedTemplate {

		// unescape the quotes if they appear
		body := strings.Replace(object.Spec.Properties.String(), `\"`, `"`, -1)
		t, err := templateEngine("str://"+body, object, depends, scope)
		if err != nil {
			// Even if the Properties contains no template functions, we still expect it
			// to be loaded as text correctly.  So an error here is an exception.
			return nil, err
		}

		view, err := t.Render(id)
		if err != nil {
			return nil, err
		}

		// if properties have been populated by the template, then it will be the default onto which
		// the spec Properties overlay on top of.
		if err := types.AnyString(view).Decode(&properties); err != nil {
			return nil, err
		}
	}

	// now encode the properties
	return types.AnyValue(properties)
}

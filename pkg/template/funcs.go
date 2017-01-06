package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jmespath/go-jmespath"
)

// DefaultFuncs returns a list of default functions for binding in the template
func (t *Template) DefaultFuncs() map[string]interface{} {
	return map[string]interface{}{
		"var": func(name, doc string, v ...interface{}) interface{} {
			if found, has := t.binds[name]; has {
				return found
			}
			return v // default
		},

		"global": func(name string, v interface{}) interface{} {
			t.binds[name] = v
			return ""
		},

		"q": func(q string, o interface{}) (interface{}, error) {
			query, err := jmespath.Compile(q)
			if err != nil {
				return nil, err
			}
			return query.Search(o)
		},

		"jsonEncode": func(o interface{}) (string, error) {
			buff, err := json.MarshalIndent(o, "", "  ")
			return string(buff), err
		},

		"jsonDecode": func(o interface{}) (interface{}, error) {
			ret := map[string]interface{}{}
			switch o := o.(type) {
			case string:
				err := json.Unmarshal([]byte(o), &ret)
				return ret, err
			case []byte:
				err := json.Unmarshal(o, &ret)
				return ret, err
			}
			return ret, fmt.Errorf("not-supported-value-type")
		},

		"include": func(p string, opt ...interface{}) (string, error) {
			var o interface{}
			if len(opt) > 0 {
				o = opt[0]
			}
			loc, err := getURL(t.url, p)
			if err != nil {
				return "", err
			}
			included, err := NewTemplate(loc, t.options)
			if err != nil {
				return "", err
			}
			// copy the binds in the parent scope into the child
			for k, v := range t.binds {
				included.binds[k] = v
			}
			// inherit the functions defined for this template
			for k, v := range t.funcs {
				included.AddFunc(k, v)
			}
			return included.Render(o)
		},

		"lines": func(o interface{}) ([]string, error) {
			ret := []string{}
			switch o := o.(type) {
			case string:
				return strings.Split(o, "\n"), nil
			case []byte:
				return strings.Split(string(o), "\n"), nil
			}
			return ret, fmt.Errorf("not-supported-value-type")
		},
	}
}

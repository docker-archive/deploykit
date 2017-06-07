package cli

import (
	"github.com/docker/infrakit/pkg/cli/backend"
	"github.com/docker/infrakit/pkg/template"
)

// loadBackend determines the backend to use for executing the rendered template text (e.g. run in shell).
// During this phase, the template delimiters are changed to =% %= so put this in the comment {{/* */}}
func (c *Context) loadBackends(t *template.Template) error {

	added := []string{}

	backend.Visit(
		func(funcName string, backend backend.TemplateFunc) {
			t.AddFunc(funcName,
				func(opt ...interface{}) error {
					executor, err := backend(c.plugins, opt...)
					if err != nil {
						return err
					}
					c.run = executor
					return nil
				})

			added = append(added, funcName)
		})
	_, err := t.Render(c)

	// clean up after we rendered...  remove the functions
	t.RemoveFunc(added...)
	return err
}

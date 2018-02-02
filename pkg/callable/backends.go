package callable

import (
	"github.com/docker/infrakit/pkg/callable/backend"
	"github.com/docker/infrakit/pkg/template"
)

// loadBackend determines the backend to use for executing the rendered template text (e.g. run in shell).
// During this phase, the template delimiters are changed to =% %= so put this in the comment {{/* */}}
func (c *Callable) loadBackends(t *template.Template) error {

	added := []string{}

	backend.VisitBackends(
		func(funcName string, backend backend.ExecFuncBuilder) {
			t.AddFunc(funcName,
				func(opt ...interface{}) error {
					executor, err := backend(c.scope, *c.test, opt...)
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

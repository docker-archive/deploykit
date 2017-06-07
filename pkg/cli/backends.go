package cli

import (
	"github.com/docker/infrakit/pkg/cli/backend"
	"github.com/docker/infrakit/pkg/template"

	_ "github.com/docker/infrakit/pkg/cli/backend/instance"
	_ "github.com/docker/infrakit/pkg/cli/backend/manager"
	_ "github.com/docker/infrakit/pkg/cli/backend/print"
	_ "github.com/docker/infrakit/pkg/cli/backend/sh"
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

	// t.AddFunc("instanceProvision",
	// 	func(name string, isYAML bool) string {
	// 		c.run = func(script string) error {

	// 			// locate the plugin
	// 			endpoint, err := c.plugins().Find(plugin.Name(name))
	// 			if err != nil {
	// 				return err
	// 			}

	// 			plugin, err := instance_plugin.NewClient(plugin.Name(name), endpoint.Address)
	// 			if err != nil {
	// 				return err
	// 			}

	// 			spec := instance.Spec{}
	// 			if isYAML {
	// 				y, err := types.AnyYAML([]byte(script))
	// 				if err != nil {
	// 					return err
	// 				}
	// 				if err := y.Decode(&spec); err != nil {
	// 					return err
	// 				}
	// 			} else {
	// 				if err := types.AnyString(script).Decode(&spec); err != nil {
	// 					return err
	// 				}
	// 			}

	// 			id, err := plugin.Provision(spec)
	// 			if err != nil {
	// 				return err
	// 			}
	// 			fmt.Println(*id)
	// 			return nil
	// 		}
	// 		return ""
	// 	})

	// t.AddFunc("managerCommit",
	// 	func(isYAML, pretend bool) string {
	// 		c.run = func(script string) error {

	// 			groups := []plugin.Spec{}
	// 			if isYAML {
	// 				y, err := types.AnyYAML([]byte(script))
	// 				if err != nil {
	// 					return err
	// 				}
	// 				if err := y.Decode(&groups); err != nil {
	// 					return err
	// 				}
	// 			} else {
	// 				if err := types.AnyString(script).Decode(&groups); err != nil {
	// 					return err
	// 				}
	// 			}

	// 			// Check the list of plugins
	// 			for _, gp := range groups {

	// 				endpoint, err := c.plugins().Find(gp.Plugin)
	// 				if err != nil {
	// 					return err
	// 				}

	// 				// unmarshal the group spec
	// 				spec := group.Spec{}
	// 				if gp.Properties != nil {
	// 					err = gp.Properties.Decode(&spec)
	// 					if err != nil {
	// 						return err
	// 					}
	// 				}

	// 				target, err := group_plugin.NewClient(endpoint.Address)

	// 				log.Debug("commit", "plugin", gp.Plugin, "address", endpoint.Address, "err", err, "spec", spec)

	// 				if err != nil {
	// 					return err
	// 				}

	// 				plan, err := target.CommitGroup(spec, pretend)
	// 				if err != nil {
	// 					return err
	// 				}

	// 				fmt.Println("Group", spec.ID, "with plugin", gp.Plugin, "plan:", plan)
	// 			}
	// 			return nil
	// 		}
	// 		return ""
	// 	})

	// t.AddFunc("sh",
	// 	func(opts ...string) string {
	// 		c.run = func(script string) error {

	// 			cmd := strings.Join(append([]string{"/bin/sh"}, opts...), " ")
	// 			log.Debug("sh", "cmd", cmd)

	// 			run := exec.Command(cmd)
	// 			run.InheritEnvs(true).StartWithHandlers(
	// 				func(stdin io.Writer) error {
	// 					_, err := stdin.Write([]byte(script))
	// 					return err
	// 				},
	// 				func(stdout io.Reader) error {
	// 					_, err := io.Copy(os.Stdout, stdout)
	// 					return err
	// 				},
	// 				func(stderr io.Reader) error {
	// 					_, err := io.Copy(os.Stderr, stderr)
	// 					return err
	// 				},
	// 			)
	// 			return run.Wait()
	// 		}
	// 		return ""
	// 	})

	_, err := t.Render(c)

	// clean up after we rendered...  remove the functions
	t.RemoveFunc(added...)
	return err
}

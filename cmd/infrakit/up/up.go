package up

import (
	"fmt"
	"time"

	"github.com/docker/infrakit/cmd/infrakit/base"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run"
	run_manager "github.com/docker/infrakit/pkg/run/manager"
	group_kind "github.com/docker/infrakit/pkg/run/v0/group"
	manager_kind "github.com/docker/infrakit/pkg/run/v0/manager"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/up")

func init() {
	base.Register(Command)
}

// Command is the entrypoint
func Command(plugins func() discovery.Plugins) *cobra.Command {

	templateFlags, toJSON, _, processTemplate := base.TemplateProcessor(plugins)

	loadRules := func(url string) ([]launch.Rule, error) {
		rules := []launch.Rule{}
		if buff, err := processTemplate(url); err != nil {
			return nil, err
		} else if view, err := toJSON([]byte(buff)); err != nil {
			return nil, err
		} else if err = types.AnyBytes(view).Decode(&rules); err != nil {
			return nil, err
		}
		return rules, nil
	}
	loadSpecs := func(url string) ([]types.Spec, error) {
		specs := []types.Spec{}
		if buff, err := processTemplate(url); err != nil {
			return nil, err
		} else if view, err := toJSON([]byte(buff)); err != nil {
			return nil, err
		} else if err = types.AnyBytes(view).Decode(&specs); err != nil {
			return nil, err
		}
		return specs, nil
	}

	up := &cobra.Command{
		Use:   "up <url>",
		Short: "Up everything",
	}

	launchConfigURL := up.Flags().String("launch-config-url", "", "URL for the startup configs")
	up.Flags().AddFlagSet(templateFlags)

	up.RunE = func(c *cobra.Command, args []string) error {

		if plugins == nil {
			panic("no plugins()")
		}

		if len(args) == 0 {
			return fmt.Errorf("missing url arg")
		}

		// Now the actual spec of the infrastructure to stand up
		specsURL := args[0]

		launchRules := []launch.Rule{}
		// parse the launch rules if any
		if *launchConfigURL != "" {
			list, err := loadRules(*launchConfigURL)
			if err != nil {
				return err
			}
			launchRules = list
		}

		// Plugin launcher runs asynchronously
		pluginManager, err := run_manager.ManagePlugins(launchRules, plugins, false, 5*time.Second)
		if err != nil {
			return err
		}
		defer pluginManager.Stop()

		// start up the basics
		err = pluginManager.Launch(inproc.ExecName, manager_kind.Kind, plugin.Name(manager_kind.LookupName), nil)
		if err != nil {
			return err
		}
		err = pluginManager.Launch(inproc.ExecName, group_kind.Kind, plugin.Name(group_kind.LookupName), nil)
		if err != nil {
			return err
		}

		pluginManager.WaitStarting()

		log.Info("Entering main loop")

		tick := time.Tick(5 * time.Second)
		stop := make(chan struct{})

		go func() {

		main:
			for {
				select {
				case <-tick:

				// commit the specs to the manager
				case <-stop:
					log.Info("Stopped checking for config changes")
					return
				}

				// refresh the specs from the url
				log.Info("Checking", "url", specsURL)
				specs, err := loadSpecs(specsURL)
				if err != nil {
					log.Error("Error loading specs", "url", specsURL, "err", err)
					continue main
				}

				// from the specs get the plugin names and kind to start
				log.Debug("Loaded specs", "specs", specs, "V", logutil.V(200))
				err = pluginManager.StartPluginsFromSpecs(specs,
					func(err error) bool {
						log.Error("cannot start plugin", "err", err)
						return false
					})

				if err != nil {
					log.Error("Error from input. Not committing.", "err", err)
					continue main
				}

				// Now tell the manager to enforce
				err = run.Call(plugins, manager.InterfaceSpec, nil,
					func(m manager.Manager) error {
						log.Debug("Calling manager to enforce", "m", m, "specs", specs)
						return m.Enforce(specs)
					})
				if err != nil {
					log.Error("Error making call to manager", "err", err)
				}

			}
		}()

		pluginManager.WaitForAllShutdown()
		log.Info("All plugins shutdown")
		close(stop)
		return nil
	}

	return up
}

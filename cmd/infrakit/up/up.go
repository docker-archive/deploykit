package up

import (
	"fmt"
	"strings"
	"time"

	"github.com/docker/infrakit/cmd/infrakit/base"
	"github.com/docker/infrakit/cmd/infrakit/manager/schema"

	"github.com/docker/infrakit/pkg/cli"
	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/run/scope/local"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/up")

func init() {
	base.Register(Command)
}

// Command is the entrypoint
func Command(scp scope.Scope) *cobra.Command {

	services := cli.NewServices(scp)

	up := &cobra.Command{
		Use:   "up <url>",
		Short: "Up everything",
	}

	waitDuration := up.Flags().String("wait", "1s", "Wait for plugins to be ready")
	configURL := up.Flags().String("config-url", "", "URL for the startup configs")
	stack := up.Flags().String("stack", "mystack", "Name of the stack")

	up.Flags().AddFlagSet(services.ProcessTemplateFlags)
	metadatas := up.Flags().StringSlice("metadata", []string{}, "key=value to set metadata")

	up.RunE = func(c *cobra.Command, args []string) error {

		if len(args) == 0 {
			return fmt.Errorf("missing url arg")
		}

		pluginManager, err := cli.PluginManager(scp, services, *configURL)
		if err != nil {
			return err
		}

		wait := types.MustParseDuration(*waitDuration)

		log.Info("Starting up base plugins")
		baseStack := fmt.Sprintf("manager:%v", *stack)
		basePlugins := []string{"vars", "group:group-stateless", baseStack}
		for _, base := range basePlugins {
			execName, kind, name, _ := local.StartPlugin(base).Parse()
			err := pluginManager.Launch(execName, kind, name, nil)
			if err != nil {
				log.Error("cannot start base plugin", "spec", base)
				return err
			}
		}
		pluginManager.WaitStarting()
		<-time.After(wait.Duration())

		if len(*metadatas) > 0 {
			log.Info("Setting metadata entries")
			mfunc := scope.MetadataFunc(scp)
			for _, md := range *metadatas {
				// TODO -- this is not transactional.... we don't know
				// the paths and there may be changes to multiple metadata
				// plugins.  For now we just process one by one.
				kv := strings.Split(md, "=")
				if len(kv) == 2 {
					_, err := mfunc(kv[0], kv[1])
					if err != nil {
						return err
					}
					log.Info("written metadata", "key", kv[0], "value", kv[1])
				}
			}
		}

		log.Info("Parsing the input groups.json as template")
		input, err := services.ReadFromStdinIfElse(
			func() bool { return args[0] == "-" },
			func() (string, error) { return services.ProcessTemplate(args[0]) },
			services.ToJSON,
		)
		if err != nil {
			log.Error("processing input", "err", err)
			return err
		}

		targets := []local.StartPlugin{}
		err = schema.ParseInputSpecs([]byte(input),
			func(name plugin.Name, id group.ID, s group_types.Spec) error {

				more, err := local.Plugins(id, s)
				if err != nil {
					return err
				}
				targets = append(targets, more...)
				return nil
			})
		if err != nil {
			log.Error("parsing input", "err", err)
			return err
		}

		return local.Execute(scp.Plugins, pluginManager,
			func() ([]local.StartPlugin, error) {
				log.Info("plugins to start", "targets", targets)
				return targets, nil
			},
			func(_ scope.Scope) error {
				// TODO - in here loop and commit periodically

				pluginManager.WaitForAllShutdown()

				return nil
			},
			local.Options{
				StartWait: wait,
				StopWait:  wait,
			},
		)
	}

	return up
}

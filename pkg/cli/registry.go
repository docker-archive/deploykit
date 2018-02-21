package cli

import (
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/spf13/cobra"
)

// CmdBuilder is a factory function that creates a command
type CmdBuilder func(name string, services *Services) *cobra.Command

var (
	lock sync.Mutex

	cmdBuilders = map[string][]CmdBuilder{}
)

// Register registers a command from the CmdBuilders
func Register(spi spi.InterfaceSpec, builders []CmdBuilder) {

	lock.Lock()
	defer lock.Unlock()

	list, has := cmdBuilders[spi.Encode()]
	if !has {
		list = []CmdBuilder{}
	}
	cmdBuilders[spi.Encode()] = append(list, builders...)
}

// visitCommands iterate through all the CmdBuilders known
func visitCommands(spi spi.InterfaceSpec, visit func(b CmdBuilder)) {
	if builders, has := cmdBuilders[spi.Encode()]; has {
		for _, builder := range builders {
			visit(builder)
		}
	}
}

func getPluginObjects(hs rpc.Handshaker, major string) map[string][]spi.InterfaceSpec {

	objects := map[string][]spi.InterfaceSpec{}

	// The spi this object implements (e.g. Instance/0.5.0)
	// For each spi, eg. Instance/0.5.0 a list of object names
	typesBySpi, err := hs.Hello()
	if err != nil {
		log.Debug("error getting typed objects in this plugin", "name", major, "err", err)

		return objects
	}

	for theSpi, objs := range typesBySpi {

		objectName := major
		for _, object := range objs {

			minor := object.Name

			if minor != "." {
				objectName = path.Join(major, minor)
			}

			if list, has := objects[objectName]; !has {
				objects[objectName] = []spi.InterfaceSpec{
					theSpi,
				}
			} else {
				objects[objectName] = append(list, theSpi)
			}
		}
	}

	return objects
}

// LoadAll loads all the dynamic, plugin commands based on what's registered and discovered.
func LoadAll(services *Services) ([]*cobra.Command, error) {
	lock.Lock()
	defer lock.Unlock()

	// first discovery all the running plugins
	running, err := services.Scope.Plugins().List()
	if err != nil {
		return nil, err
	}

	type object struct {
		name    string
		cmd     *cobra.Command
		actions map[action]CmdBuilder
	}

	all := map[string]object{}

	// Show the interfaces implemented by each plugin
	for major, entry := range running {
		hs, err := client.NewHandshaker(entry.Address)
		if err != nil {
			log.Debug("handshaker error", "err", err, "addr", entry.Address)
			continue
		}

		// for each object, we have a name and a list of interfaces.
		for name, spis := range getPluginObjects(hs, major) {

			all[name] = object{
				name:    name,
				cmd:     &cobra.Command{Use: name},
				actions: map[action]CmdBuilder{},
			}

			for _, spi := range spis {
				visitCommands(spi, func(buildCmd CmdBuilder) {

					subcommand := buildCmd(name, services)
					verb := strings.Split(subcommand.Use, " ")[0]

					all[name].actions[action{spi: spi, verb: verb}] = buildCmd
				})
			}
		}
	}

	// Build the final command hierarchy.
	commands := []*cobra.Command{}

	for name, object := range all {

		interfaces := map[spi.InterfaceSpec]struct{}{}
		verbs := map[string]struct{}{}

		for action, builder := range object.actions {

			interfaces[action.spi] = struct{}{}

			cmd := builder(name, services)
			cmd.Short = fmt.Sprintf("%s (%v)", cmd.Short, action.spi.Encode())

			if _, has := overrides[action]; has {
				// TODO - replace the line below with NOT adding the command
				cmd.Use = strings.Join([]string{action.verb, strings.ToLower(action.spi.Name)}, "-")
			}

			if _, has := verbs[cmd.Use]; has {
				cmd.Use = strings.Join([]string{action.verb, strings.ToLower(action.spi.Name)}, "-")
			}

			object.cmd.AddCommand(cmd)

			verbs[cmd.Use] = struct{}{}
		}

		list := []string{}
		for k := range interfaces {
			list = append(list, k.Encode())
		}
		object.cmd.Short = fmt.Sprintf("Access object %s which implements %s", name, strings.Join(list, ", "))

		commands = append(commands, object.cmd)
	}

	return commands, nil
}

type action struct {
	spi  spi.InterfaceSpec
	verb string
}

// Overrides contains the interface and verb that wins in the case
// of same verb command appear from another interface.
// This is a crude way to implement overriding of verbs by interfaces that are
// proxies (e.g. manager or controller interfaces).
var overrides = map[action]action{

	{
		spi:  group.InterfaceSpec,
		verb: "commit",
	}: {
		spi:  controller.InterfaceSpec,
		verb: "commit",
	},

	{
		spi:  group.InterfaceSpec,
		verb: "describe",
	}: {
		spi:  controller.InterfaceSpec,
		verb: "describe",
	},
}

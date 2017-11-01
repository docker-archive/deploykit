package cli

import (
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi"
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
	spis, err := hs.Implements()
	if err != nil {
		log.Debug("error getting interface", "name", major, "err", err)
		return objects
	}

	// For each spi, eg. Instance/0.5.0 a list of object names
	typesBySpi, err := hs.Types()
	if err != nil {
		log.Debug("error getting typed objects in this plugin", "name", major, "err", err)

		// Here we assume there are no lower level objects
		objects[major] = spis
		return objects
	}

	for encodedSpi, names := range typesBySpi {

		// the key is a string form of InterfaceSpec because yaml/ json don't handle
		// objects as keys very well.

		theSpi := spi.DecodeInterfaceSpec(string(encodedSpi))

		objectName := major
		for _, minor := range names {

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

	commands := []*cobra.Command{}

	// Show the interfaces implemented by each plugin
	for major, entry := range running {
		hs, err := client.NewHandshaker(entry.Address)
		if err != nil {
			log.Debug("handshaker error", "err", err, "addr", entry.Address)
			continue
		}

		objects := getPluginObjects(hs, major)

		// for each object, we have a name and a list of interfaces.
		for name, spis := range objects {

			command := &cobra.Command{
				Use: name,
			}

			// Prevents duplicate sub-commands. This can happen when multiple
			// 'info' subcommands map to the each interface the plugin implements
			seen := map[string]*cobra.Command{}

			list := []string{}
			for _, spi := range spis {
				list = append(list, spi.Encode())

				visitCommands(spi, func(buildCmd CmdBuilder) {

					subcommand := buildCmd(name, services)

					parts := strings.Split(subcommand.Use, " ")
					verb := parts[0]

					if _, has := seen[verb]; has {
						if verb == "info" {
							return // skip this
						}
						// splice the spi into the usage.. eg. describe-group
						subcommand.Use = strings.Join(append(
							[]string{verb + "-" + strings.ToLower(spi.Name)}, parts[1:]...), " ")
					}

					command.AddCommand(subcommand)
					seen[verb] = subcommand
				})
			}

			command.Short = fmt.Sprintf("Access object %s which implements %s",
				name, strings.Join(list, ","))

			commands = append(commands, command)
		}
	}

	return commands, nil
}

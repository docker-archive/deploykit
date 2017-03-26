package main

import (
	"flag"
	"net/url"
	"os"

	"github.com/docker/infrakit/cmd/cli/event"
	"github.com/docker/infrakit/cmd/cli/flavor"
	"github.com/docker/infrakit/cmd/cli/group"
	"github.com/docker/infrakit/cmd/cli/info"
	"github.com/docker/infrakit/cmd/cli/instance"
	"github.com/docker/infrakit/cmd/cli/manager"
	"github.com/docker/infrakit/cmd/cli/metadata"
	"github.com/docker/infrakit/cmd/cli/plugin"
	"github.com/docker/infrakit/cmd/cli/resource"
	"github.com/docker/infrakit/cmd/cli/template"
	"github.com/docker/infrakit/cmd/cli/util"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/discovery/remote"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/spf13/cobra"
)

// A generic client for infrakit
func main() {

	log := logutil.New("module", "main")

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "infrakit cli",
	}

	// Log setup
	logOptions := &logutil.ProdDefaults
	ulist := []*url.URL{}
	remotes := []string{}

	cmd.PersistentFlags().AddFlagSet(cli.Flags(logOptions))
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	cmd.PersistentFlags().StringSliceVarP(&remotes, "host", "H", remotes, "host list. Default is local sockets")

	// parse the list of hosts
	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		logutil.Configure(logOptions)

		if len(remotes) > 0 {
			for _, h := range remotes {
				u, err := url.Parse(h)
				if err != nil {
					return err
				}
				ulist = append(ulist, u)
			}
		}
		return nil
	}

	// Don't print usage text for any error returned from a RunE function.
	// Only print it when explicitly requested.
	cmd.SilenceUsage = true

	// Don't automatically print errors returned from a RunE function.
	// They are returned from cmd.Execute() below and we print it ourselves.
	cmd.SilenceErrors = true
	f := func() discovery.Plugins {
		if len(ulist) == 0 {
			d, err := local.NewPluginDiscovery()
			if err != nil {
				log.Crit("Failed to initialize plugin discovery", "err", err)
				os.Exit(1)
			}
			return d
		}

		d, err := remote.NewPluginDiscovery(ulist)
		if err != nil {
			log.Crit("Failed to initialize remote plugin discovery", "err", err)
			os.Exit(1)
		}
		return d
	}

	cmd.AddCommand(cli.VersionCommand(),
		info.Command(f),
		template.Command(f),
		manager.Command(f),
		metadata.Command(f),
		event.Command(f),
		plugin.Command(f),
		util.Command(f),
		instance.Command(f),
		group.Command(f),
		flavor.Command(f),
		resource.Command(f))

	usage := banner + "\n\n" + cmd.UsageTemplate()
	cmd.SetUsageTemplate(usage)

	err := cmd.Execute()
	if err != nil {
		log.Crit("error executing", "err", err)
		os.Exit(1)
	}
}

const banner = `
 ___  ________   ________ ________  ________  ___  __    ___  _________   
|\  \|\   ___  \|\  _____\\   __  \|\   __  \|\  \|\  \ |\  \|\___   ___\ 
\ \  \ \  \\ \  \ \  \__/\ \  \|\  \ \  \|\  \ \  \/  /|\ \  \|___ \  \_| 
 \ \  \ \  \\ \  \ \   __\\ \   _  _\ \   __  \ \   ___  \ \  \   \ \  \  
  \ \  \ \  \\ \  \ \  \_| \ \  \\  \\ \  \ \  \ \  \\ \  \ \  \   \ \  \ 
   \ \__\ \__\\ \__\ \__\   \ \__\\ _\\ \__\ \__\ \__\\ \__\ \__\   \ \__\
    \|__|\|__| \|__|\|__|    \|__|\|__|\|__|\|__|\|__| \|__|\|__|    \|__|
`

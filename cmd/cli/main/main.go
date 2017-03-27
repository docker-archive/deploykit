package main

import (
	"flag"
	"net/url"
	"os"

	"github.com/docker/infrakit/cmd/cli/base"
	"github.com/docker/infrakit/pkg/cli"
	cli_local "github.com/docker/infrakit/pkg/cli/local"
	"github.com/docker/infrakit/pkg/discovery"
	discovery_local "github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/discovery/remote"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/spf13/cobra"
)

func init() {
	logutil.Configure(&logutil.ProdDefaults)
}

// A generic client for infrakit
func main() {

	if err := discovery_local.Setup(); err != nil {
		panic(err)
	}
	if err := cli_local.Setup(); err != nil {
		panic(err)
	}

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
			d, err := discovery_local.NewPluginDiscovery()
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

	cmd.AddCommand(cli.VersionCommand())

	base.VisitModules(f, func(c *cobra.Command) {
		cmd.AddCommand(c)
	})

	// additional modules
	modules, err := cli_local.NewModules(cli_local.Dir())
	if err != nil {
		log.Crit("error executing", "err", err)
		os.Exit(1)
	}

	mods, err := modules.List()
	log.Debug("modules", "mods", mods)
	if err != nil {
		log.Crit("error executing", "err", err)
		os.Exit(1)
	}

	for _, mod := range mods {
		log.Debug("Adding", "module", mod.Use)
		cmd.AddCommand(mod)
	}

	usage := banner + "\n\n" + cmd.UsageTemplate()
	cmd.SetUsageTemplate(usage)

	err = cmd.Execute()
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

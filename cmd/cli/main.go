package main

import (
	"errors"
	"flag"
	"net/url"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
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
			d, err := discovery.NewPluginDiscovery()
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
	cmd.AddCommand(infoCommand(f))
	cmd.AddCommand(templateCommand(f))
	cmd.AddCommand(managerCommand(f))
	cmd.AddCommand(metadataCommand(f))
	cmd.AddCommand(eventCommand(f))
	cmd.AddCommand(pluginCommand(f))
	cmd.AddCommand(utilCommand(f))

	cmd.AddCommand(instancePluginCommand(f), groupPluginCommand(f), flavorPluginCommand(f), resourcePluginCommand(f))

	err := cmd.Execute()
	if err != nil {
		log.Crit("error executing", "err", err)
		os.Exit(1)
	}
}

func assertNotNil(message string, f interface{}) {
	if f == nil {
		logutil.New("main", "assert").Error("assert failed", "err", errors.New(message))
		os.Exit(1)
	}
}

// upTree traverses up the command tree and starts executing the do function in the order from top
// of the command tree to the bottom.  Cobra commands executes only one level of PersistentPreRunE
// in reverse order.  This breaks our model of setting log levels at the very top and have the log level
// set throughout the entire hierarchy of command execution.
func upTree(c *cobra.Command, do func(*cobra.Command, []string) error) error {
	if p := c.Parent(); p != nil {
		return upTree(p, do)
	}
	return do(c, c.Flags().Args())
}

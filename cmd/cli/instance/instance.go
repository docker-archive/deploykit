package instance

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/docker/infrakit/cmd/cli/base"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/instance")

func init() {
	base.Register(Command)
}

// Command is the entry point to this module
func Command(plugins func() discovery.Plugins) *cobra.Command {

	var instancePlugin instance.Plugin

	cmd := &cobra.Command{
		Use:   "instance",
		Short: "Access instance plugin",
	}
	name := cmd.PersistentFlags().String("name", "", "Name of plugin")
	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if err := cli.EnsurePersistentPreRunE(c); err != nil {
			return err
		}

		endpoint, err := plugins().Find(plugin.Name(*name))
		if err != nil {
			return err
		}

		p, err := instance_plugin.NewClient(plugin.Name(*name), endpoint.Address)
		if err != nil {
			return err
		}
		instancePlugin = p

		cli.MustNotNil(instancePlugin, "instance plugin not found", "name", *name)
		return nil
	}

	validate := &cobra.Command{
		Use:   "validate <instance configuration file>",
		Short: "validates an instance configuration",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			buff, err := ioutil.ReadFile(args[0])
			if err != nil {
				log.Warn("error", "err", err)
				os.Exit(1)
			}

			err = instancePlugin.Validate(types.AnyBytes(buff))
			if err == nil {
				fmt.Println("validate:ok")
			}
			return err
		},
	}

	provision := &cobra.Command{
		Use:   "provision <instance configuration file>",
		Short: "provisions an instance",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			buff, err := ioutil.ReadFile(args[0])
			if err != nil {
				log.Warn("error", "err", err)
				os.Exit(1)
			}

			spec := instance.Spec{}
			if err := json.Unmarshal(buff, &spec); err != nil {
				return err
			}

			id, err := instancePlugin.Provision(spec)
			if err == nil && id != nil {
				fmt.Printf("%s\n", *id)
			}
			return err
		},
	}

	destroy := &cobra.Command{
		Use:   "destroy <instance ID>",
		Short: "destroy the resource",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			instanceID := instance.ID(args[0])
			err := instancePlugin.Destroy(instanceID)

			if err == nil {
				fmt.Println("destroyed", instanceID)
			}
			return err
		},
	}

	describe := &cobra.Command{
		Use:   "describe",
		Short: "describe the instances",
	}
	tags := describe.Flags().StringSlice("tags", []string{}, "Tags to filter")
	quiet := describe.Flags().BoolP("quiet", "q", false, "Print rows without column headers")
	describe.RunE = func(cmd *cobra.Command, args []string) error {

		filter := map[string]string{}
		for _, t := range *tags {
			p := strings.Split(t, "=")
			if len(p) == 2 {
				filter[p[0]] = p[1]
			} else {
				filter[p[0]] = ""
			}
		}

		desc, err := instancePlugin.DescribeInstances(filter)
		if err == nil {

			if !*quiet {
				fmt.Printf("%-30s\t%-30s\t%-s\n", "ID", "LOGICAL", "TAGS")
			}
			for _, d := range desc {
				logical := "  -   "
				if d.LogicalID != nil {
					logical = string(*d.LogicalID)
				}

				printTags := []string{}
				for k, v := range d.Tags {
					printTags = append(printTags, fmt.Sprintf("%s=%s", k, v))
				}
				sort.Strings(printTags)

				fmt.Printf("%-30s\t%-30s\t%-s\n", d.ID, logical, strings.Join(printTags, ","))
			}
		}

		return err
	}
	cmd.AddCommand(validate, provision, destroy, describe)

	return cmd
}

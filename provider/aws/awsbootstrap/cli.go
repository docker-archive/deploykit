package awsbootstrap

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/spi/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"io/ioutil"
	"os"
	"strconv"
)

func NewDriverCli() cli.DriverCli {
	return &awsBootstrap{}
}

type awsBootstrap struct {
}

func readConfig(swimFile string) (fakeSWIMSchema, error) {
	swim := fakeSWIMSchema{}
	swimData, err := ioutil.ReadFile(swimFile)
	if err != nil {
		return swim, fmt.Errorf("Failed to read config file: %s", err)
	}

	err = json.Unmarshal(swimData, &swim)
	if err != nil {
		return swim, err
	}

	err = swim.validate()
	if err != nil {
		return swim, err
	}

	swim.applyDefaults()

	return swim, err
}

type clusterIDFlags struct {
	ID clusterID
}

func (c *clusterIDFlags) flags() *pflag.FlagSet {
	clusterIDFlags := pflag.NewFlagSet("cluster ID", pflag.ExitOnError)
	clusterIDFlags.StringVar(&c.ID.region, "region", "", "AWS region")
	clusterIDFlags.StringVar(&c.ID.name, "cluster", "", "Machete cluster name")
	return clusterIDFlags
}

func (c *clusterIDFlags) valid() bool {
	return c.ID.region != "" && c.ID.name != ""
}

func abort(format string, args ...interface{}) {
	log.Fatalf(format, args...)
	os.Exit(1)
}

func (a awsBootstrap) Command() *cobra.Command {
	cmd := cobra.Command{Use: "aws"}

	cmd.AddCommand(&cobra.Command{
		Use:   "create <swim config>",
		Short: "perform the bootstrap sequence",
		Long:  "bootstrap a swarm cluster using a SWIM configuration",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			swim, err := readConfig(args[0])
			if err != nil {
				abort("Invalid config file: %s", err)
			}

			err = bootstrap(swim)
			if err != nil {
				abort(err.Error())
			}
		},
	})

	cluster := clusterIDFlags{}

	var swimFile string
	destroyCmd := cobra.Command{
		Use:   "destroy",
		Short: "destroy a swarm cluster",
		Long: `destroy all resources associated with a SWIM cluster

The cluster may be identified manually or based on the contents of a SWIM file.`,
		Run: func(cmd *cobra.Command, args []string) {
			var id clusterID
			if swimFile == "" {
				if !cluster.valid() {
					abort("Must specify --config or both of --region and --cluster")
				}

				id = cluster.ID
			} else {
				swim, err := readConfig(swimFile)
				if err != nil {
					abort("Invalid config file: %s", err)
				}
				id = swim.cluster()
			}

			err := destroy(id)
			if err != nil {
				abort(err.Error())
			}
		},
	}
	destroyCmd.Flags().StringVar(&swimFile, "config", "", "A SWIM file")
	destroyCmd.Flags().AddFlagSet(cluster.flags())
	cmd.AddCommand(&destroyCmd)

	scaleCmd := cobra.Command{
		Use:   "scale <region> <cluster> <group> <target count>",
		Short: "adjust the target instance count in a scaling group",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 4 {
				cmd.Usage()
				os.Exit(1)
			}

			// TODO(wfarner): Since this command must be run from one of the managers, we should have a
			// mechanism to infer the SWIM config location from the engine (and omit the arg).

			cluster := clusterID{region: args[0], name: args[1]}
			groupName := args[2]
			targetCount, err := strconv.Atoi(args[3])
			if err != nil {
				abort("target count must be an integer")
			}
			if targetCount <= 0 {
				abort("target count must be greater than zero")
			}

			err = scale(cluster, groupName, targetCount)
			if err != nil {
				abort(err.Error())
			}
		},
	}
	cmd.AddCommand(&scaleCmd)

	cmd.AddCommand(&cobra.Command{
		Use: "reconfigure <swim config>",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			swim, err := readConfig(args[0])
			if err != nil {
				abort("Invalid config file: %s", err)
			}

			// TODO(wfarner): Fetch the existing config and check that the requested change is possible.

			err = swim.push()
			if err != nil {
				abort("Failed to push config: %s", err)
			}
			log.Infof("Configuration pushed")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use: "describe <swim config>",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			swim, err := readConfig(args[0])
			if err != nil {
				abort("Invalid config file: %s", err)
			}

			groups := []string{}
			for name := range swim.Groups {
				groups = append(groups, name)
			}

			log.Infof("Groups: %s", groups)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use: "status",
		Run: func(cmd *cobra.Command, args []string) {
			// TODO(wfarner): Implement.

			log.Infof("Managers: 3 instances")
			log.Infof("Workers: 5 instances")
		},
	})

	return &cmd
}

type logger struct {
}

func (l logger) Log(args ...interface{}) {
	log.Println(args)
}

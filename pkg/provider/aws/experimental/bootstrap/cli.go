package bootstrap

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/docker/infrakit.aws/plugin/instance"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"io/ioutil"
	"os"
)

// NewCLI creates a CLI.
func NewCLI() *CLI {
	return &CLI{}
}

// CLI is a CLI for AWS bootstrapping.
type CLI struct {
}

func readConfig(clusterSpecFile string) (clusterSpec, error) {
	spec := clusterSpec{}
	specData, err := ioutil.ReadFile(clusterSpecFile)
	if err != nil {
		return spec, fmt.Errorf("Failed to read config file: %s", err)
	}

	err = json.Unmarshal(specData, &spec)
	if err != nil {
		return spec, err
	}

	err = spec.validate()
	if err != nil {
		return spec, err
	}

	spec.applyDefaults()

	return spec, err
}

type clusterIDFlags struct {
	ID clusterID
}

func (c *clusterIDFlags) flags() *pflag.FlagSet {
	clusterIDFlags := pflag.NewFlagSet("cluster ID", pflag.ExitOnError)
	clusterIDFlags.StringVar(&c.ID.region, "region", "", "AWS region")
	clusterIDFlags.StringVar(&c.ID.name, "cluster", "", "Infrakit cluster name")
	return clusterIDFlags
}

func (c *clusterIDFlags) valid() bool {
	return c.ID.region != "" && c.ID.name != ""
}

func abort(format string, args ...interface{}) {
	log.Fatalf(format, args...)
	os.Exit(1)
}

// AddCommands attaches subcommands for the CLI.
func (a *CLI) AddCommands(root *cobra.Command) {
	cluster := clusterIDFlags{}

	var keyName string

	workerSize := 3

	createCmd := cobra.Command{
		Use:   "create [<cluster config>]",
		Short: "create a swarm cluster",
		Run: func(cmd *cobra.Command, args []string) {
			spec := clusterSpec{}
			if len(args) == 1 {
				if keyName != "" || cluster.ID.name != "" || cluster.ID.region != "" {
					abort("No other cluster-related flags may be set when a spec file is used")
				}

				var err error
				spec, err = readConfig(args[0])
				if err != nil {
					abort("Invalid config file: %s", err)
				}
			} else {
				if keyName == "" || !cluster.valid() {
					abort("When creating from flags, --key, --cluster, and --region must be provided")
				}

				instanceConfig := instance.CreateInstanceRequest{
					RunInstancesInput: ec2.RunInstancesInput{
						ImageId: aws.String("ami-d4fe5fb4"),
						KeyName: aws.String(keyName),
						Placement: &ec2.Placement{
							// TODO(wfarner): Picking the AZ like this feels hackish.
							AvailabilityZone: aws.String(cluster.ID.region + "a"),
						},
					},
				}

				spec = clusterSpec{
					ClusterName: cluster.ID.name,
					Groups: []instanceGroupSpec{
						{
							Name:   group.ID("Managers"),
							Type:   managerType,
							Size:   3,
							Config: instanceConfig,
						},
						{
							Name:   group.ID("Workers"),
							Type:   workerType,
							Size:   workerSize,
							Config: instanceConfig,
						},
					},
				}

				err := spec.validate()
				if err != nil {
					abort(err.Error())
				}

				spec.applyDefaults()
			}

			err := bootstrap(spec)
			if err != nil {
				abort(err.Error())
			}
		},
	}
	createCmd.Flags().AddFlagSet(cluster.flags())
	createCmd.Flags().StringVar(&keyName, "key", "", "The existing SSH key in AWS to use for provisioned instances")
	createCmd.Flags().IntVar(&workerSize, "worker_size", workerSize, "Size of worker group")

	root.AddCommand(&createCmd)

	var clusterSpec string
	destroyCmd := cobra.Command{
		Use:   "destroy",
		Short: "destroy a swarm cluster",
		Long: `destroy all resources associated with a cluster

The cluster may be identified manually or based on the contents of a cluster spec file.`,
		Run: func(cmd *cobra.Command, args []string) {
			var id clusterID
			if clusterSpec == "" {
				if !cluster.valid() {
					abort("Must specify --config or both of --region and --cluster")
				}

				id = cluster.ID
			} else {
				spec, err := readConfig(clusterSpec)
				if err != nil {
					abort("Invalid config file: %s", err)
				}
				id = spec.cluster()
			}

			err := destroy(id)
			if err != nil {
				abort(err.Error())
			}
		},
	}
	destroyCmd.Flags().StringVar(&clusterSpec, "config", "", "A cluster spec file")

	destroyCmd.Flags().AddFlagSet(cluster.flags())
	root.AddCommand(&destroyCmd)
}

type logger struct {
}

func (l logger) Log(args ...interface{}) {
	log.Println(args)
}

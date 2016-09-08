package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provider/aws"
	"github.com/docker/libmachete/spi"
	"github.com/spf13/cobra"
)

func awsCommand(backend *backend) *cobra.Command {

	// TOOD(chungers) -- no reason why a cluster id needs be passed to an instance driver.
	// The instance provisioner uses cluster id only for tagging.  We should move that out of the instance provisioner.
	cluster := "default"

	builder := &aws.Builder{}

	aws := &cobra.Command{
		Use:   "aws",
		Short: "Runs the AWS instance provisioner plugin",
		RunE: func(_ *cobra.Command, args []string) error {

			provisioner, err := builder.BuildInstanceProvisioner(spi.ClusterID(cluster))
			if err != nil {
				log.Error(err)
				return err
			}

			backend.plugin = provisioner
			return nil
		},
	}

	aws.Flags().StringVar(&cluster, "cluster", cluster,
		"Machete cluster ID, used to isolate separate infrastructures")

	// TODO(chungers) - the exposed flags here won't be set in plugins, because plugin install doesn't allow
	// user to pass in command line args like containers with entrypoint.
	aws.Flags().AddFlagSet(builder.Flags())

	return aws
}

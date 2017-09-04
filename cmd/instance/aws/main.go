package main

import (
	"os"

	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	aws_plugin "github.com/docker/infrakit/pkg/provider/aws/plugin"
	"github.com/docker/infrakit/pkg/provider/aws/plugin/instance"
	event_rpc "github.com/docker/infrakit/pkg/rpc/event"
	instance_rpc "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/spi/event"
	instance_spi "github.com/docker/infrakit/pkg/spi/instance"
	"github.com/spf13/cobra"
)

func main() {

	builder := &instance.Builder{}

	var logLevel int
	var name string
	var namespaceTags []string
	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "AWS instance plugin",
		Run: func(c *cobra.Command, args []string) {

			namespace := map[string]string{}
			for _, tagKV := range namespaceTags {
				keyAndValue := strings.Split(tagKV, "=")
				if len(keyAndValue) != 2 {
					log.Error("Namespace tags must be formatted as key=value")
					os.Exit(1)
				}

				namespace[keyAndValue[0]] = keyAndValue[1]
			}

			instancePlugin, err := builder.BuildInstancePlugin(namespace)
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			autoscalingClient := autoscaling.New(builder.Config)
			cloudWatchLogsClient := cloudwatchlogs.New(builder.Config)
			dynamodbClient := dynamodb.New(builder.Config)
			ec2Client := ec2.New(builder.Config)
			elbClient := elb.New(builder.Config)
			iamClient := iam.New(builder.Config)
			sqsClient := sqs.New(builder.Config)

			cli.SetLogLevel(logLevel)
			run.Plugin(plugin.DefaultTransport(name),
				// As event plugin
				event_rpc.PluginServerWithTypes(
					map[string]event.Plugin{
						"ec2-instance": (&instance.Monitor{
							Plugin: instancePlugin,
						}).Init(),
					}),

				// instance plugins
				instance_rpc.PluginServerWithTypes(map[string]instance_spi.Plugin{
					"autoscaling-autoscalinggroup":    instance.NewAutoScalingGroupPlugin(autoscalingClient, namespace),
					"autoscaling-launchconfiguration": instance.NewLaunchConfigurationPlugin(autoscalingClient, namespace),
					"cloudwatchlogs-loggroup":         instance.NewLogGroupPlugin(cloudWatchLogsClient, namespace),
					"dynamodb-table":                  instance.NewTablePlugin(dynamodbClient, namespace),
					"ec2-instance":                    instancePlugin,
					"ec2-spot-instance":               instance.NewSpotInstancePlugin(ec2Client, namespace),
					"ec2-internetgateway":             instance.NewInternetGatewayPlugin(ec2Client, namespace),
					"ec2-routetable":                  instance.NewRouteTablePlugin(ec2Client, namespace),
					"ec2-securitygroup":               instance.NewSecurityGroupPlugin(ec2Client, namespace),
					"ec2-subnet":                      instance.NewSubnetPlugin(ec2Client, namespace),
					"ec2-volume":                      instance.NewVolumePlugin(ec2Client, namespace),
					"ec2-vpc":                         instance.NewVpcPlugin(ec2Client, namespace),
					"elb-loadbalancer":                instance.NewLoadBalancerPlugin(elbClient, namespace),
					"iam-instanceprofile":             instance.NewInstanceProfilePlugin(iamClient, namespace),
					"iam-role":                        instance.NewRolePlugin(iamClient, namespace),
					"sqs-queue":                       instance.NewQueuePlugin(sqsClient, namespace),
				}))
		},
	}

	cmd.Flags().IntVar(&logLevel, "log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	cmd.Flags().StringVar(&name, "name", "instance-aws", "Plugin name to advertise for discovery")
	cmd.Flags().StringSliceVar(
		&namespaceTags,
		"namespace-tags",
		[]string{},
		"A list of key=value resource tags to namespace all resources created")

	// TODO(chungers) - the exposed flags here won't be set in plugins, because plugin install doesn't allow
	// user to pass in command line args like containers with entrypoint.
	cmd.Flags().AddFlagSet(builder.Flags())

	cmd.AddCommand(aws_plugin.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

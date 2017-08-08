package aws

import (
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	aws "github.com/docker/infrakit/pkg/provider/aws/plugin/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// CanonicalName is the canonical name of the plugin for starting up, etc.
	CanonicalName = "instance-aws"
)

var (
	log = logutil.New("module", "run/instance/aws")
)

func init() {
	inproc.Register(CanonicalName, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Name of the plugin
	Name string

	// Namespace is a set of kv pairs for tags that namespaces the resource instances
	Namespace map[string]string

	aws.Options `json:",inline" yaml:",inline"`
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Name:      CanonicalName,
	Namespace: map[string]string{},
	Options: aws.Options{
		Region: "us-west-1",
	},
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(plugins func() discovery.Plugins,
	config *types.Any) (name plugin.Name, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	builder := aws.Builder{Options: options.Options}

	var instancePlugin instance.Plugin
	instancePlugin, err = builder.BuildInstancePlugin(options.Namespace)
	if err != nil {
		return
	}
	autoscalingClient := autoscaling.New(builder.Config)
	cloudWatchLogsClient := cloudwatchlogs.New(builder.Config)
	dynamodbClient := dynamodb.New(builder.Config)
	ec2Client := ec2.New(builder.Config)
	elbClient := elb.New(builder.Config)
	iamClient := iam.New(builder.Config)
	sqsClient := sqs.New(builder.Config)

	name = plugin.Name(options.Name)
	impls = map[run.PluginCode]interface{}{
		run.Event: map[string]event.Plugin{
			"ec2-instance": (&aws.Monitor{Plugin: instancePlugin}).Init(),
		},
		run.Instance: map[string]instance.Plugin{
			"autoscaling-autoscalinggroup":    aws.NewAutoScalingGroupPlugin(autoscalingClient, options.Namespace),
			"autoscaling-launchconfiguration": aws.NewLaunchConfigurationPlugin(autoscalingClient, options.Namespace),
			"cloudwatchlogs-loggroup":         aws.NewLogGroupPlugin(cloudWatchLogsClient, options.Namespace),
			"dynamodb-table":                  aws.NewTablePlugin(dynamodbClient, options.Namespace),
			"ec2-instance":                    instancePlugin,
			"ec2-internetgateway":             aws.NewInternetGatewayPlugin(ec2Client, options.Namespace),
			"ec2-routetable":                  aws.NewRouteTablePlugin(ec2Client, options.Namespace),
			"ec2-securitygroup":               aws.NewSecurityGroupPlugin(ec2Client, options.Namespace),
			"ec2-subnet":                      aws.NewSubnetPlugin(ec2Client, options.Namespace),
			"ec2-volume":                      aws.NewVolumePlugin(ec2Client, options.Namespace),
			"ec2-vpc":                         aws.NewVpcPlugin(ec2Client, options.Namespace),
			"elb-loadbalancer":                aws.NewLoadBalancerPlugin(elbClient, options.Namespace),
			"iam-instanceprofile":             aws.NewInstanceProfilePlugin(iamClient, options.Namespace),
			"iam-role":                        aws.NewRolePlugin(iamClient, options.Namespace),
			"sqs-queue":                       aws.NewQueuePlugin(sqsClient, options.Namespace),
		},
	}
	return
}

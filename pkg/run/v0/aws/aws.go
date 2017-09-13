package aws

import (
	"strings"
	"time"

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
	aws_instance "github.com/docker/infrakit/pkg/provider/aws/plugin/instance"
	aws_loadbalancer "github.com/docker/infrakit/pkg/provider/aws/plugin/loadbalancer"
	aws_metadata "github.com/docker/infrakit/pkg/provider/aws/plugin/metadata"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "aws"

	// EnvRegion is the env for aws region.  Don't set this if want auto detect.
	EnvRegion = "INFRAKIT_AWS_REGION"

	// EnvStackName is the env for stack name
	EnvStackName = "INFRAKIT_AWS_STACKNAME"

	// EnvMetadataTemplateURL is the location of the template for Metadata plugin
	EnvMetadataTemplateURL = "INFRAKIT_AWS_METADATA_TEMPLATE_URL"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_AWS_NAMESPACE_TAGS"

	// EnvELBName is the name of the ELB ENV variable name for the ELB plugin.
	EnvELBName = "INFRAKIT_AWS_ELB_NAME"
)

var (
	log = logutil.New("module", "run/v0/aws")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Namespace is a set of kv pairs for tags that namespaces the resource instances
	Namespace map[string]string

	aws_metadata.Options `json:",inline" yaml:",inline"`
}

func defaultNamespace() map[string]string {
	t := map[string]string{}
	list := local.Getenv(EnvNamespaceTags, "")
	for _, v := range strings.Split(list, ",") {
		p := strings.Split(v, "=")
		if len(p) == 2 {
			t[p[0]] = p[1]
		}
	}
	return t
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Namespace: defaultNamespace(),
	Options: aws_metadata.Options{
		Template:  local.Getenv(EnvMetadataTemplateURL, ""),
		StackName: local.Getenv(EnvStackName, ""),
		Options: aws_instance.Options{
			Region: local.Getenv(EnvRegion, ""), // empty string trigger auto-detect
		},
		PollInterval: 60 * time.Second,
	},
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(plugins func() discovery.Plugins, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	var metadataPlugin metadata.Plugin
	stopMetadataPlugin := make(chan struct{})
	metadataPlugin, err = aws_metadata.NewPlugin(options.Options, stopMetadataPlugin)
	if err != nil {
		return
	}

	onStop = func() { close(stopMetadataPlugin) }

	var instancePlugin instance.Plugin
	builder := aws_instance.Builder{Options: options.Options.Options}
	instancePlugin, err = builder.BuildInstancePlugin(options.Namespace)
	if err != nil {
		return
	}

	var elbPlugin loadbalancer.L4
	elbClient := elb.New(builder.Config)
	elbPlugin, err = aws_loadbalancer.NewELBPlugin(elbClient, local.Getenv(EnvELBName, "default"))
	if err != nil {
		return
	}

	autoscalingClient := autoscaling.New(builder.Config)
	cloudWatchLogsClient := cloudwatchlogs.New(builder.Config)
	dynamodbClient := dynamodb.New(builder.Config)
	ec2Client := ec2.New(builder.Config)
	iamClient := iam.New(builder.Config)
	sqsClient := sqs.New(builder.Config)

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Event: map[string]event.Plugin{
			"ec2-instance": (&aws_instance.Monitor{Plugin: instancePlugin}).Init(),
		},
		run.Metadata: metadataPlugin,
		run.L4: func() (map[string]loadbalancer.L4, error) {
			return map[string]loadbalancer.L4{
				local.Getenv(EnvELBName, "default"): elbPlugin,
			}, nil
		},
		run.Instance: map[string]instance.Plugin{
			"autoscaling-autoscalinggroup":    aws_instance.NewAutoScalingGroupPlugin(autoscalingClient, options.Namespace),
			"autoscaling-launchconfiguration": aws_instance.NewLaunchConfigurationPlugin(autoscalingClient, options.Namespace),
			"cloudwatchlogs-loggroup":         aws_instance.NewLogGroupPlugin(cloudWatchLogsClient, options.Namespace),
			"dynamodb-table":                  aws_instance.NewTablePlugin(dynamodbClient, options.Namespace),
			"ec2-instance":                    instancePlugin,
			"ec2-spot-instance":               aws_instance.NewSpotInstancePlugin(ec2Client, options.Namespace),
			"ec2-internetgateway":             aws_instance.NewInternetGatewayPlugin(ec2Client, options.Namespace),
			"ec2-routetable":                  aws_instance.NewRouteTablePlugin(ec2Client, options.Namespace),
			"ec2-securitygroup":               aws_instance.NewSecurityGroupPlugin(ec2Client, options.Namespace),
			"ec2-subnet":                      aws_instance.NewSubnetPlugin(ec2Client, options.Namespace),
			"ec2-volume":                      aws_instance.NewVolumePlugin(ec2Client, options.Namespace),
			"ec2-vpc":                         aws_instance.NewVpcPlugin(ec2Client, options.Namespace),
			"elb-loadbalancer":                aws_instance.NewLoadBalancerPlugin(elbClient, options.Namespace),
			"iam-instanceprofile":             aws_instance.NewInstanceProfilePlugin(iamClient, options.Namespace),
			"iam-role":                        aws_instance.NewRolePlugin(iamClient, options.Namespace),
			"sqs-queue":                       aws_instance.NewQueuePlugin(sqsClient, options.Namespace),
		},
	}

	return
}

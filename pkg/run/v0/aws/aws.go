package aws

import (
	"strings"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	aws_instance "github.com/docker/infrakit/pkg/provider/aws/plugin/instance"
	aws_loadbalancer "github.com/docker/infrakit/pkg/provider/aws/plugin/loadbalancer"
	aws_metadata "github.com/docker/infrakit/pkg/provider/aws/plugin/metadata"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
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
	EnvStackName = "INFRAKIT_AWS_STACK_NAME"

	// EnvMetadataTemplateURL is the location of the template for Metadata plugin
	EnvMetadataTemplateURL = "INFRAKIT_AWS_METADATA_TEMPLATE_URL"

	// EnvMetadataPollInterval is the env to set fo polling for metadata updates
	EnvMetadataPollInterval = "INFRAKIT_AWS_METADATA_POLL_INTERVAL"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_AWS_NAMESPACE_TAGS"

	// EnvELBNames is the name of the ELB ENV variable name for the ELB plugin.
	EnvELBNames = "INFRAKIT_AWS_ELB_NAMES"
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

	// ELBNames is a list of names for ELB instances to start the L4 plugins
	ELBNames []string

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
	ELBNames:  strings.Split(local.Getenv(EnvELBNames, ""), ","),
	Options: aws_metadata.Options{
		Template:  local.Getenv(EnvMetadataTemplateURL, ""),
		StackName: local.Getenv(EnvStackName, ""),
		Options: aws_instance.Options{
			Region:          local.Getenv(EnvRegion, ""), // empty string trigger auto-detect
			AccessKeyID:     local.Getenv("AWS_ACCESS_KEY_ID", ""),
			SecretAccessKey: local.Getenv("AWS_SECRET_ACCESS_KEY", ""),
		},
		PollInterval: types.MustParseDuration(local.Getenv(EnvMetadataPollInterval, "60s")),
	},
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(scope scope.Scope, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	var instancePlugin instance.Plugin
	builder := aws_instance.Builder{Options: options.Options.Options}
	instancePlugin, err = builder.BuildInstancePlugin(options.Namespace)
	if err != nil {
		return
	}

	ec2Client := ec2.New(builder.Config)
	iamClient := iam.New(builder.Config)
	elbClient := elb.New(builder.Config)

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: map[string]instance.Plugin{
			"ec2-instance":        instancePlugin,
			"ec2-spot-instance":   aws_instance.NewSpotInstancePlugin(ec2Client, options.Namespace),
			"ec2-internetgateway": aws_instance.NewInternetGatewayPlugin(ec2Client, options.Namespace),
			"ec2-routetable":      aws_instance.NewRouteTablePlugin(ec2Client, options.Namespace),
			"ec2-securitygroup":   aws_instance.NewSecurityGroupPlugin(ec2Client, options.Namespace),
			"ec2-subnet":          aws_instance.NewSubnetPlugin(ec2Client, options.Namespace),
			"ec2-volume":          aws_instance.NewVolumePlugin(ec2Client, options.Namespace),
			"ec2-vpc":             aws_instance.NewVpcPlugin(ec2Client, options.Namespace),
			"elb-loadbalancer":    aws_instance.NewLoadBalancerPlugin(elbClient, options.Namespace),
			"iam-instanceprofile": aws_instance.NewInstanceProfilePlugin(iamClient, options.Namespace),
			"iam-role":            aws_instance.NewRolePlugin(iamClient, options.Namespace),
		},
	}

	// Expose ELBs
	l4Map := map[string]loadbalancer.L4{}
	for _, elbName := range options.ELBNames {
		var elbPlugin loadbalancer.L4
		elbPlugin, err = aws_loadbalancer.NewELBPlugin(elbClient, elbName)
		if err != nil {
			return
		}
		l4Map[elbName] = elbPlugin
	}

	if len(l4Map) > 0 {
		impls[run.L4] = func() (map[string]loadbalancer.L4, error) { return l4Map, nil }
	}

	if u := local.Getenv(EnvMetadataTemplateURL, ""); u != "" {

		log.Info("Include metadata plugin", "url", u)

		var metadataPlugin metadata.Plugin
		stopMetadataPlugin := make(chan struct{})
		metadataPlugin, err = aws_metadata.NewPlugin(options.Options, stopMetadataPlugin)
		if err != nil {
			return
		}
		impls[run.Metadata] = metadataPlugin

		cleanup := onStop
		onStop = func() {
			close(stopMetadataPlugin)
			if cleanup != nil {
				cleanup()
			}
		}
	}

	return
}

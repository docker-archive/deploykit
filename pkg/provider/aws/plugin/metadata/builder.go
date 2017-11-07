package metadata

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/docker/infrakit/pkg/provider/aws/plugin/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/pflag"
)

// Options containe properties important for the AWS api
type Options struct {
	instance.Options `json:",inline" yaml:",inline"`

	Debug           bool
	Template        string
	TemplateOptions template.Options
	StackName       string
	PollInterval    types.Duration
}

// Flags returns the flags required.
func (options *Options) Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("aws", pflag.PanicOnError)
	flags.BoolVar(&options.Debug, "api-debug", false, "True to turn on API debugging")
	flags.StringVar(&options.Region, "region", "", "AWS region")
	flags.StringVar(&options.AccessKeyID, "access-key-id", "", "IAM access key ID")
	flags.StringVar(&options.SecretAccessKey, "secret-access-key", "", "IAM access key secret")
	flags.StringVar(&options.SessionToken, "session-token", "", "AWS STS token")
	flags.IntVar(&options.Retries, "retries", 5, "Number of retries for AWS API operations")
	return flags
}

// NewPlugin creates an instance of the plugin
func NewPlugin(options Options, stop <-chan struct{}) (*Context, error) {

	providers := []credentials.Provider{
		&ec2rolecreds.EC2RoleProvider{Client: ec2metadata.New(session.New())},
		&credentials.EnvProvider{},
		&credentials.SharedCredentialsProvider{},
	}
	if (len(options.AccessKeyID) > 0 && len(options.SecretAccessKey) > 0) || len(options.SessionToken) > 0 {
		staticCreds := credentials.StaticProvider{
			Value: credentials.Value{
				AccessKeyID:     options.AccessKeyID,
				SecretAccessKey: options.SecretAccessKey,
				SessionToken:    options.SessionToken,
			},
		}
		providers = append(providers, &staticCreds)
	}

	if options.Region == "" || options.Region == "auto" {
		log.Warn("region not specified, attempting to discover from EC2 instance metadata")
		region, err := instance.GetRegion()
		if err != nil {
			return nil, errors.New("Unable to determine region")
		}

		log.Info("Defaulting to local region", "region", region)
		options.Region = region
	}

	config := aws.NewConfig().
		WithRegion(options.Region).
		WithCredentials(credentials.NewChainCredentials(providers)).
		WithLogger(GetLogger()).
		WithMaxRetries(options.Retries)
	if options.Debug {
		config.WithLogLevel(aws.LogDebugWithRequestErrors)
	}
	session := session.New(config)

	context := &Context{
		templateURL:     options.Template,
		templateOptions: options.TemplateOptions,
		poll:            options.PollInterval.Duration(),
		stop:            stop,
		stackName:       options.StackName,
		clients: AWSClients{
			Cfn: cloudformation.New(session),
			Ec2: ec2.New(session),
			Asg: autoscaling.New(session),
		},
	}

	context.start()
	return context, nil

}

type logger struct {
}

func (l logger) Log(args ...interface{}) {
	log.Debug("log", "args", args)
}

// GetLogger gets a logger that can be used with the AWS SDK.
func GetLogger() aws.Logger {
	return &logger{}
}

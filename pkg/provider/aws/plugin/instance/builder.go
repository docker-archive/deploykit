package instance

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/spf13/pflag"
)

// Options contain the options for aws plugin
type Options struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Retries         int
}

// Builder is a ProvisionerBuilder that creates an AWS instance provisioner.
type Builder struct {
	Config  client.ConfigProvider
	Options Options
}

// Flags returns the flags required.
func (b *Builder) Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("aws", pflag.PanicOnError)
	flags.StringVar(&b.Options.Region, "region", "", "AWS region")
	flags.StringVar(&b.Options.AccessKeyID, "access-key-id", "", "IAM access key ID")
	flags.StringVar(&b.Options.SecretAccessKey, "secret-access-key", "", "IAM access key secret")
	flags.StringVar(&b.Options.SessionToken, "session-token", "", "AWS STS token")
	flags.IntVar(&b.Options.Retries, "retries", 5, "Number of retries for AWS API operations")
	return flags
}

// BuildInstancePlugin creates an instance Provisioner configured with the Flags.
func (b *Builder) BuildInstancePlugin(namespaceTags map[string]string) (instance.Plugin, error) {
	if b.Config == nil {
		providers := []credentials.Provider{
			&ec2rolecreds.EC2RoleProvider{Client: ec2metadata.New(session.New())},
			&credentials.EnvProvider{},
			&credentials.SharedCredentialsProvider{},
		}

		if (len(b.Options.AccessKeyID) > 0 && len(b.Options.SecretAccessKey) > 0) || len(b.Options.SessionToken) > 0 {
			staticCreds := credentials.StaticProvider{
				Value: credentials.Value{
					AccessKeyID:     b.Options.AccessKeyID,
					SecretAccessKey: b.Options.SecretAccessKey,
					SessionToken:    b.Options.SessionToken,
				},
			}
			providers = append(providers, &staticCreds)
		}

		if b.Options.Region == "" || b.Options.Region == "auto" {
			log.Warn("region not specified, attempting to discover from EC2 instance metadata")
			region, err := GetRegion()
			if err != nil {
				return nil, errors.New("Unable to determine region")
			}

			log.Warn("Defaulting to local region", "region", region)
			b.Options.Region = region
		}

		b.Config = session.New(aws.NewConfig().
			WithRegion(b.Options.Region).
			WithCredentials(credentials.NewChainCredentials(providers)).
			WithLogger(GetLogger()).
			//WithLogLevel(aws.LogDebugWithRequestErrors).
			WithMaxRetries(b.Options.Retries))
	}

	return NewInstancePlugin(ec2.New(b.Config), namespaceTags), nil
}

type logger struct {
}

func (l logger) Log(args ...interface{}) {
	log.Info("AWS SDK", "message", fmt.Sprint(args...))
}

// GetLogger gets a logger that can be used with the AWS SDK.
func GetLogger() aws.Logger {
	return &logger{}
}

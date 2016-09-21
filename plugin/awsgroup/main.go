// This is a demo program for creating ana manading groups in AWS.  It will no longer be necessary once
// plugin discovery is implemented.

package main

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	machete_aws "github.com/docker/libmachete.aws"
	"github.com/docker/libmachete/plugin/group/groupserver"
	"github.com/docker/libmachete/spi/instance"
)

func main() {
	pluginLookup := func(key string) (instance.Plugin, error) {
		switch key {
		case "aws":
			providers := []credentials.Provider{
				&ec2rolecreds.EC2RoleProvider{Client: ec2metadata.New(session.New())},
				&credentials.EnvProvider{},
				&credentials.SharedCredentialsProvider{},
			}

			region, err := machete_aws.GetRegion()
			if err != nil {
				return nil, fmt.Errorf("Failed to determine local region: %s", err)
			}

			client := session.New(aws.NewConfig().
				WithRegion(region).
				WithCredentials(credentials.NewChainCredentials(providers)).
				WithLogger(machete_aws.GetLogger()).
				WithMaxRetries(3))

			return machete_aws.NewInstancePlugin(ec2.New(client)), nil
		default:
			return nil, errors.New("Unknown instance plugin")
		}
	}

	groupserver.Run(pluginLookup)
}

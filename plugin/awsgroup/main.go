// This is a demo program for creating and managing groups in AWS.  It will no longer be necessary once
// plugin discovery is implemented.

package main

import (
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	machete_aws "github.com/docker/libmachete.aws"
	"github.com/docker/libmachete/plugin/group/groupserver"
	"github.com/docker/libmachete/spi/instance"
	"github.com/spf13/cobra"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

var (
	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "awsgroup",
		Short: "Create and manage groups of Swarm machines in AWS.",
		Long:  "",
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print build version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s (revision %s)\n", Version, Revision)
		},
	})

	var regionOption string
	pluginLookup := func(key string) (instance.Plugin, error) {
		switch key {
		case "aws":
			providers := []credentials.Provider{
				&ec2rolecreds.EC2RoleProvider{Client: ec2metadata.New(session.New())},
				&credentials.EnvProvider{},
				&credentials.SharedCredentialsProvider{},
			}

			var region string
			if regionOption == "" {
				var err error
				region, err = machete_aws.GetRegion()
				if err != nil {
					return nil, fmt.Errorf("Failed to determine local region: %s", err)
				}
			} else {
				region = regionOption
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

	var port uint
	runCmd := cobra.Command{
		Use:   "run",
		Short: "run the awsgroup HTTP server",
		Long: `Runs an HTTP server that manages machine groups.

Once the server is running, other subcommands may be used as a convenience to interact with groups`,
		Run: func(cmd *cobra.Command, args []string) {
			groupserver.Run(port, pluginLookup)
		},
	}
	runCmd.Flags().UintVar(&port, "port", 8888, "Port the server listens on")
	runCmd.Flags().StringVar(
		&regionOption,
		"region",
		"",
		"AWS region to use, overrides value from instance metadata")

	rootCmd.AddCommand(&runCmd)

	sendRequest := func(addr string, path string, body io.Reader) {
		resp, err := http.Post(fmt.Sprintf("%s/%s", addr, path), "application/json", body)
		if err != nil {
			log.Fatalf("Request failed: %s", err)
			os.Exit(1)
		}

		responseBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Failed to read response body: %s", err)
			os.Exit(1)
		}

		if len(responseBody) != 0 {
			log.Infof("Response: %s", string(responseBody))
		}

		if resp.StatusCode != 200 {
			os.Exit(1)
		}
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "watch <awsgroup address> <group config>",
		Short: "adds a group to be watched by the awsgroup server",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 2 {
				rootCmd.Usage()
				os.Exit(1)
			}

			config, err := os.Open(args[1])
			if err != nil {
				log.Fatalf("Failed to read group config: %s", err)
				os.Exit(1)
			}

			sendRequest(args[0], "Watch", config)
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "unwatch <awsgroup address> <group ID>",
		Short: "instructs the awsgroup server to stop watching a group",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 2 {
				rootCmd.Usage()
				os.Exit(1)
			}

			sendRequest(args[0], fmt.Sprintf("Unwatch/%s", args[1]), nil)
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "inspect <awsgroup address> <group ID>",
		Short: "shows details about a group",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 2 {
				rootCmd.Usage()
				os.Exit(1)
			}

			sendRequest(args[0], fmt.Sprintf("Inspect/%s", args[1]), nil)
		},
	})

	var describe bool
	updateCmd := cobra.Command{
		Use:   "update <awsgroup address> <group config>",
		Short: "shows details about a group",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 2 {
				rootCmd.Usage()
				os.Exit(1)
			}

			config, err := os.Open(args[1])
			if err != nil {
				log.Fatalf("Failed to read group config: %s", err)
				os.Exit(1)
			}

			var path string
			if describe {
				path = "DescribeUpdate"
			} else {
				path = "UpdateGroup"
			}

			sendRequest(args[0], path, config)
		},
	}
	updateCmd.Flags().BoolVar(&describe, "describe", false, "Only show details about what the update will do")

	rootCmd.AddCommand(&updateCmd)

	rootCmd.AddCommand(&cobra.Command{
		Use:   "destroy <awsgroup address> <group ID>",
		Short: "destroys a group and all instances contained within it",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 2 {
				rootCmd.Usage()
				os.Exit(1)
			}

			sendRequest(args[0], fmt.Sprintf("DestroyGroup/%s", args[1]), nil)
		},
	})

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

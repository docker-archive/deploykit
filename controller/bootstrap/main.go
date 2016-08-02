package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	machete_aws "github.com/docker/libmachete/provider/aws"
	"github.com/spf13/cobra"
	"io/ioutil"
	"log"
	"os"
	"strconv"
)

type instanceConfig struct {
	InstanceType string `json:"instance_type"`
	Count        int    `json:"count"`
}

type config struct {
	Region   string         `json:"region"`
	KeyName  string         `json:"key_name"`
	Workers  instanceConfig `json:"workers"`
	Managers instanceConfig `json:"managers"`
}

var (
	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

func run(templateFile, configStr string) {
	templateData, err := ioutil.ReadFile(templateFile)
	if err != nil {
		log.Printf("Failed ot read config file: %s", err)
		os.Exit(1)
	}

	config := config{}
	err = json.Unmarshal([]byte(configStr), &config)
	if err != nil {
		log.Printf("Configuration contains invalid json: %s", err.Error())
		os.Exit(1)
	}

	providers := []credentials.Provider{
		&ec2rolecreds.EC2RoleProvider{Client: ec2metadata.New(session.New())},
		&credentials.EnvProvider{},
		&credentials.SharedCredentialsProvider{},
	}

	// If the region was not specified, try to detect it if running within AWS.
	if config.Region == "" {
		region, err := machete_aws.GetRegion()
		if err != nil {
			log.Printf("Failed to determine region: %s", err)
			os.Exit(1)
		}
		config.Region = region
	}

	sess := session.New(aws.NewConfig().
		WithRegion(config.Region).
		WithCredentialsChainVerboseErrors(true).
		WithCredentials(credentials.NewChainCredentials(providers)).
		WithLogger(getLogger()).
		WithLogLevel(aws.LogDebugWithRequestErrors))

	cfn := cloudformation.New(sess)

	stack := cloudformation.CreateStackInput{
		StackName:    aws.String("bill-stack"),
		Tags:         []*cloudformation.Tag{{Key: aws.String("machete"), Value: aws.String("testing")}},
		TemplateBody: aws.String(string(templateData)),
		Capabilities: []*string{
			aws.String("CAPABILITY_IAM"),
		},
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("KeyName"),
				ParameterValue: &config.KeyName,
			},
			{
				ParameterKey:   aws.String("InstanceType"),
				ParameterValue: &config.Workers.InstanceType,
			},
			{
				ParameterKey:   aws.String("ManagerInstanceType"),
				ParameterValue: &config.Managers.InstanceType,
			},
			{
				ParameterKey:   aws.String("ClusterSize"),
				ParameterValue: aws.String(strconv.Itoa(config.Workers.Count)),
			},
			{
				ParameterKey:   aws.String("ManagerSize"),
				ParameterValue: aws.String(strconv.Itoa(config.Managers.Count)),
			},
		},
	}

	output, err := cfn.CreateStack(&stack)
	if err != nil {
		log.Printf("Failed to create stack: %s", err)
		os.Exit(1)
	}

	log.Printf("Successfully created stack %s", *output.StackId)
}

func main() {
	rootCmd := &cobra.Command{
		Use: "bootstrap",
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "run <template> <parameters>",
		Short: "perform the bootstrap sequence",
		Long:  "bootstrap a swarm cluster using a CloudFormation template and parameters",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 2 {
				cmd.Usage()
				return
			}

			templateFile := args[0]
			configStr := args[1]

			run(templateFile, configStr)
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print the bootstrap version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s (revision %s)\n", Version, Revision)
		},
	})

	err := rootCmd.Execute()
	if err != nil {
		log.Print(err)
		os.Exit(-1)
	}
}

type logger struct {
	logger *log.Logger
}

func (l logger) Log(args ...interface{}) {
	l.logger.Println(args...)
}

func getLogger() aws.Logger {
	return &logger{
		logger: log.New(os.Stderr, "", log.LstdFlags),
	}
}

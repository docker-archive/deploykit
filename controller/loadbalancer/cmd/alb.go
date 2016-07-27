package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provider/azure"
	"github.com/docker/libmachete/spi/loadbalancer"
	"github.com/spf13/cobra"
)

func albCommand() *cobra.Command {
	albOptions := &azure.Options{}

	cmd := &cobra.Command{
		Use:   "alb",
		Short: "Ops on the ALB (Azure Load Balancer)",
	}
	cmd.Flags().IntVar(&albOptions.PollingDelay,
		"polling_delay", 5, "Polling delay")
	cmd.Flags().StringVar(&albOptions.Environment,
		"environment", "", "environment")
	cmd.Flags().StringVar(&albOptions.OAuthClientID,
		"oauth_client_id", "", "OAuth client ID")
	cmd.Flags().StringVar(&albOptions.SubscriptionID,
		"subscription_id", "", "subscription ID")
	cmd.Flags().StringVar(&albOptions.ResourceGroupName,
		"resource_group", "", "resource group name")

	cmd.Flags().StringVar(&albOptions.ADClientID,
		"ad_app_id", "", "AD app ID")
	cmd.Flags().StringVar(&albOptions.ADClientSecret,
		"ad_app_secret", "", "AD app secret")

	describeCmd := &cobra.Command{
		Use:   "describe",
		Short: "Describes the ALB",
		RunE: func(_ *cobra.Command, args []string) error {
			log.Infoln("Running describe ALB")

			name := args[0]

			cred := azure.NewCredential()

			err := cred.Authenticate(*albOptions)
			if err != nil {
				return err
			}

			client, err := azure.CreateALBClient(cred, *albOptions)
			if err != nil {
				return err
			}

			p, err := azure.NewLoadBalancerDriver(client, albOptions.ResourceGroupName, name)
			if err != nil {
				return err
			}

			backends, err := p.Backends()
			if err != nil {
				return err
			}
			fmt.Println(backends)
			return nil
		},
	}

	publishOptions := new(struct {
		ExtPort     uint32
		BackendPort uint32
		Protocol    string
	})

	publishCmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a service at given ports",
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]

			cred := azure.NewCredential()

			err := cred.Authenticate(*albOptions)
			if err != nil {
				return err
			}

			client, err := azure.CreateALBClient(cred, *albOptions)
			if err != nil {
				return err
			}

			p, err := azure.NewLoadBalancerDriver(client, albOptions.ResourceGroupName, name)
			if err != nil {
				return err
			}

			result, err := p.Publish(loadbalancer.Route{
				Port:             publishOptions.BackendPort,
				Protocol:         loadbalancer.ProtocolFromString(publishOptions.Protocol),
				LoadBalancerPort: publishOptions.ExtPort,
			})
			if err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	}
	publishCmd.Flags().Uint32Var(&publishOptions.ExtPort, "ext_port", 80, "External port")
	publishCmd.Flags().Uint32Var(&publishOptions.BackendPort, "backend_port", 30000, "Backend port")
	publishCmd.Flags().StringVar(&publishOptions.Protocol, "protocol", "http", "Protocol: http|https|tcp|tls")

	unpublishPort := uint32(80)
	unpublishCmd := &cobra.Command{
		Use:   "unpublish",
		Short: "Unpublish a service at given port",
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]

			cred := azure.NewCredential()

			err := cred.Authenticate(*albOptions)
			if err != nil {
				return err
			}

			client, err := azure.CreateALBClient(cred, *albOptions)
			if err != nil {
				return err
			}

			p, err := azure.NewLoadBalancerDriver(client, albOptions.ResourceGroupName, name)
			if err != nil {
				return err
			}

			result, err := p.Unpublish(unpublishPort)
			if err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	}
	unpublishCmd.Flags().Uint32Var(&unpublishPort, "ext_port", unpublishPort, "External port")

	cmd.AddCommand(describeCmd, publishCmd, unpublishCmd)

	return cmd
}

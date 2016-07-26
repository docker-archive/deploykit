package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provider/aws"
	"github.com/docker/libmachete/spi/loadbalancer"
	"github.com/spf13/cobra"
)

func elbCommand() *cobra.Command {

	elbOptions := new(aws.Options)

	cmd := &cobra.Command{
		Use:   "elb",
		Short: "Ops on the ELB",
	}
	cmd.Flags().IntVar(&elbOptions.Retries,
		"retries", 10, "Retries")
	cmd.Flags().StringVar(&elbOptions.Region,
		"region", "", "Region")

	describeCmd := &cobra.Command{
		Use:   "describe",
		Short: "Describes the ELB",
		RunE: func(_ *cobra.Command, args []string) error {
			log.Infoln("Running describe ELB")

			name := args[0]

			p, err := aws.NewLoadBalancerDriver(aws.CreateELBClient(aws.Credentials(nil), *elbOptions), name)
			if err != nil {
				return err
			}
			result, err := p.Backends()
			if err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	}

	registerCmd := &cobra.Command{
		Use:   "register",
		Short: "Register an instance",
		RunE: func(_ *cobra.Command, args []string) error {
			log.Infoln("Running register instance for ELB")

			name := args[0]
			instanceID := args[1]

			p, err := aws.NewLoadBalancerDriver(aws.CreateELBClient(aws.Credentials(nil), *elbOptions), name)
			if err != nil {
				return err
			}
			result, err := p.RegisterBackend(instanceID)
			if err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	}

	deregisterCmd := &cobra.Command{
		Use:   "deregister",
		Short: "De-register an instance",
		RunE: func(_ *cobra.Command, args []string) error {
			log.Infoln("Running register instance for ELB")

			name := args[0]
			instanceID := args[1]

			p, err := aws.NewLoadBalancerDriver(aws.CreateELBClient(aws.Credentials(nil), *elbOptions), name)
			if err != nil {
				return err
			}
			result, err := p.DeregisterBackend(instanceID)
			if err != nil {
				return err
			}
			fmt.Println(result)
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

			p, err := aws.NewLoadBalancerDriver(aws.CreateELBClient(aws.Credentials(nil), *elbOptions), name)
			if err != nil {
				return err
			}

			result, err := p.Publish(loadbalancer.Backend{
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

			p, err := aws.NewLoadBalancerDriver(aws.CreateELBClient(aws.Credentials(nil), *elbOptions), name)
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

	cmd.AddCommand(describeCmd, registerCmd, deregisterCmd, publishCmd, unpublishCmd)

	return cmd
}

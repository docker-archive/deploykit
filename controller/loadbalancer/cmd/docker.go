package main

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/docker/libmachete/controller/loadbalancer"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func dockerClient(host string, tlsOptions tlsconfig.Options) (client.APIClient, error) {
	tls := &tlsOptions
	if tlsOptions.KeyFile == "" || tlsOptions.CAFile == "" || tlsOptions.CertFile == "" {
		log.Infoln("Not using TLS")
		tls = nil
	}
	return loadbalancer.NewDockerClient(host, tls)
}

func dockerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker",
		Short: "Operation with the Docker engine",
	}

	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Engine info",
		RunE: func(_ *cobra.Command, args []string) error {

			log.Infoln("Connecting to docker:", host)
			client, err := dockerClient(host, tlsOptions)
			if err != nil {
				return err
			}

			info, err := client.Info(context.Background())
			if err != nil {
				return err
			}

			buff, err := json.MarshalIndent(info, "  ", "  ")
			if err != nil {
				return err
			}

			fmt.Println(string(buff))

			return nil
		},
	}

	serviceListOptions := &types.ServiceListOptions{}
	serviceListCmd := &cobra.Command{
		Use:   "services",
		Short: "Swarm services",
		RunE: func(_ *cobra.Command, args []string) error {

			log.Infoln("Connecting to docker:", host)
			client, err := dockerClient(host, tlsOptions)
			if err != nil {
				return err
			}

			list, err := client.ServiceList(context.Background(), *serviceListOptions)
			if err != nil {
				return err
			}

			buff, err := json.MarshalIndent(list, "  ", "  ")
			if err != nil {
				return err
			}

			fmt.Println(string(buff))

			return nil
		},
	}

	nodeListOptions := &types.NodeListOptions{}
	nodeListCmd := &cobra.Command{
		Use:   "nodes",
		Short: "Swarm nodes",
		RunE: func(_ *cobra.Command, args []string) error {

			log.Infoln("Connecting to docker:", host)
			client, err := dockerClient(host, tlsOptions)
			if err != nil {
				return err
			}

			list, err := client.NodeList(context.Background(), *nodeListOptions)
			if err != nil {
				return err
			}

			buff, err := json.MarshalIndent(list, "  ", "  ")
			if err != nil {
				return err
			}

			fmt.Println(string(buff))

			return nil
		},
	}

	taskListCmd := &cobra.Command{
		Use:   "tasks",
		Short: "Swarm tasks for service",
		RunE: func(_ *cobra.Command, args []string) error {

			if len(args) == 0 {
				return fmt.Errorf("need service name")
			}

			ctx := context.Background()

			service := args[0]

			log.Infoln("Connecting to docker:", host)
			client, err := dockerClient(host, tlsOptions)
			if err != nil {
				return err
			}

			svc, _, err := client.ServiceInspectWithRaw(ctx, service)
			if err != nil {
				return err
			}

			filter := filters.Args{}
			filter.Add("service", svc.ID)
			filter.Add("desired_state", string(swarm.TaskStateRunning))
			filter.Add("desired_state", string(swarm.TaskStateAccepted))

			list, err := client.TaskList(context.Background(), types.TaskListOptions{Filter: filter})
			if err != nil {
				return err
			}

			buff, err := json.MarshalIndent(list, "  ", "  ")
			if err != nil {
				return err
			}

			fmt.Println(string(buff))

			return nil
		},
	}

	cmd.AddCommand(infoCmd, serviceListCmd, nodeListCmd, taskListCmd)

	return cmd
}

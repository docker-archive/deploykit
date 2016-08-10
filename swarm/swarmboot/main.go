package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/spf13/cobra"
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
		Use: "swarmboot",
		Long: `swarmboot initializes a group of Docker Engines in Swarm manager mode.

		The IP address of the boot leader must be specified, which is used to determine how swarmboot
		behaves.  The node that self-identifes as the boot leader will run the equivalent of
		'docker swarm init'.  Other nodes will join the leader node.`,
	}

	var dockerSocket string
	getDockerClient := func() (*client.Client, error) {
		return client.NewClient(
			fmt.Sprintf("unix://%s", dockerSocket),
			"v1.24",
			nil,
			map[string]string{})
	}

	var worker bool

	initCmd := cobra.Command{
		Use:   "init",
		Short: "begin the swarmboot sequence by initializing the cluster",
		Run: func(cmd *cobra.Command, args []string) {
			localDocker, err := getDockerClient()
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}

			err = initializeSwarm(localDocker)
			if err != nil {
				log.Fatal(err.Error())
				os.Exit(1)
			}
		},
	}
	initCmd.Flags().StringVar(&dockerSocket, "docker-socket", "/var/run/docker.sock", "Docker daemon socket path")

	joinCmd := cobra.Command{
		Use:   "join <IP>",
		Short: "join a cluster with the given IP",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			joinIP := args[0]

			localDocker, err := getDockerClient()
			if err != nil {
				log.Fatal(err.Error())
				os.Exit(1)
			}

			err = joinSwarm(localDocker, joinIP, worker)
			if err != nil {
				log.Fatal(err.Error())
				os.Exit(1)
			}
		},
	}
	joinCmd.Flags().StringVar(&dockerSocket, "docker-socket", "/var/run/docker.sock", "Docker daemon socket path")
	joinCmd.Flags().BoolVar(&worker, "worker", false, "If not the leader, whether to join as a worker node")

	rootCmd.AddCommand(&initCmd)
	rootCmd.AddCommand(&joinCmd)
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print build version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s (revision %s)\n", Version, Revision)
		},
	})

	err := rootCmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(-1)
	}
}

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
)

var (
	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

type swimPart struct {
	ManagerIPs []string
}

func fetchSWIM(url string) (*swimPart, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch SWIM file: %s", err)
	}

	swimData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read SWIM data: %s", err)
	}

	swim := swimPart{}
	err = json.Unmarshal(swimData, &swim)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal SWIM data: %s", err)
	}

	if swim.ManagerIPs == nil || len(swim.ManagerIPs) == 0 {
		return nil, errors.New("SWIM file does not list any ManagerIPs")
	}

	return &swim, nil
}

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

	runCmd := cobra.Command{
		Use:   "run <MY IP> <SWIM URL>",
		Short: "join or initialize a swarm cluster",
		Long: `join a swarm cluster whose details can be found at the given URL
		This instance will be identified with the provided <MY IP> IP address.  If this node's IP address is
		a manager, join as a manager node (or initialize if this is the manager IP listed first).`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 2 {
				cmd.Usage()
				os.Exit(1)
			}

			myIP := args[0]
			swimURL := args[1]

			swim, err := fetchSWIM(swimURL)
			if err != nil {
				log.Fatal(err)
				os.Exit(1)
			}

			manager := false
			for _, ip := range swim.ManagerIPs {
				if myIP == ip {
					manager = true
					break
				}
			}

			localDocker, err := getDockerClient()
			if err != nil {
				log.Fatal(err)
				os.Exit(1)
			}

			bootLeaderIP := swim.ManagerIPs[0]

			if myIP == bootLeaderIP {
				err = initializeSwarm(localDocker)
				if err != nil {
					log.Fatal(err)
					os.Exit(1)
				}
			} else {
				err = joinSwarm(localDocker, bootLeaderIP, manager)
				if err != nil {
					log.Fatal(err)
					os.Exit(1)
				}
			}

			if manager {
				cmd := exec.Command("/bin/sh", "manager-containers.sh")
				cmd.Env = []string{
					fmt.Sprintf("SWIM_URL=%s", swimURL),
					fmt.Sprintf("LOCAL_IP=%s", myIP),
				}

				output, err := cmd.CombinedOutput()
				if err != nil {
					log.Fatalf("Failed to run container script: %s", err)
					log.Fatal(string(output))
					os.Exit(1)
				}

				log.Infof("Script output: %s", string(output))
			}
		},
	}
	runCmd.Flags().StringVar(&dockerSocket, "docker-socket", "/var/run/docker.sock", "Docker daemon socket path")

	rootCmd.AddCommand(&runCmd)
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

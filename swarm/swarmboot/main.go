package main

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

var (
	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

type instanceConfig struct {
	Type   string
	Config json.RawMessage
}

type swimPart struct {
	ManagerIPs []string
	Groups     map[string]instanceConfig
}

func fetchSWIM(url string) ([]byte, *swimPart, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to fetch SWIM file: %s", err)
	}

	swimData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to read SWIM data: %s", err)
	}

	swim := swimPart{}
	err = json.Unmarshal(swimData, &swim)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to unmarshal SWIM data: %s", err)
	}

	if swim.ManagerIPs == nil || len(swim.ManagerIPs) == 0 {
		return nil, nil, errors.New("SWIM file does not list any ManagerIPs")
	}

	return swimData, &swim, nil
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

	var joinToken string
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

			swimData, swim, err := fetchSWIM(swimURL)
			if err != nil {
				log.Fatal(err)
				os.Exit(1)
			}

			err = ioutil.WriteFile("config.swim", swimData, 0660)
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
				if joinToken == "" {
					log.Fatal("This node is not the boot leader, so --join-token must be non-empty")
					os.Exit(1)
				}

				err = joinSwarm(localDocker, bootLeaderIP, joinToken)
				if err != nil {
					log.Fatal(err)
					os.Exit(1)
				}
			}

			if manager {
				swarmStatus, err := localDocker.SwarmInspect(context.Background())

				// TODO(wfarner): This will need to change if/when we support multiple groups of the
				// same type.
				for _, config := range swim.Groups {
					var token string
					switch config.Type {
					case "manager":
						token = swarmStatus.JoinTokens.Manager

					case "worker":
						token = swarmStatus.JoinTokens.Worker

					default:
						log.Errorf("Unknown group type %s", config.Type)
						os.Exit(1)
					}

					result := strings.Replace(
						string(config.Config),
						"{{.JOIN_TOKEN_ARG}}",
						fmt.Sprintf("--join-token %s", token),
						-1)

					err = ioutil.WriteFile(
						fmt.Sprintf("/scratch/%s-request.swpt", config.Type),
						[]byte(result),
						0660)
					if err != nil {
						log.Fatal(err)
						os.Exit(1)
					}
				}

				cmd := exec.Command("/bin/sh", "manager-containers.sh")
				cmd.Env = []string{
					fmt.Sprintf("LOCAL_IP=%s", myIP),
					fmt.Sprintf("SWIM_URL=%s", swimURL),
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
	runCmd.Flags().StringVar(
		&joinToken,
		"join-token",
		"",
		"The Swarm cluster join token. Must be set if this node is not a boot leader.")

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

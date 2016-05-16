package command

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/storage"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
)

import (
	// Load the supported provisioners
	_ "github.com/docker/libmachete/provisioners/aws"
	_ "github.com/docker/libmachete/provisioners/azure"
)

// ClientOptions encapsulates all the options for controlling client behavior
type ClientOptions struct {
	Port string
}

var (
	clientOptions = ClientOptions{}
)

// ClientCmd returns the serve subcommand.
func ClientCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "client",
		Short: "Client to the server",
	}

	cmd.PersistentFlags().StringVar(&clientOptions.Port, "port", "8888",
		"Port the server listens on. File path for unix socket")

	cmd.AddCommand(LsCmd(&clientOptions), GetCmd(&clientOptions))
	return cmd
}

// LsCmd returns the command that lists the nodes
func LsCmd(options *ClientOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list nodes",
		RunE: func(_ *cobra.Command, args []string) error {
			log.Infoln("list options = ", options, "args=", args)

			client, err := getHttpClient(options.Port)
			if err != nil {
				return err
			}
			return doList(client)
		},
	}
}

// GetCmd returns the command that lists the nodes
func GetCmd(options *ClientOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect",
		Short: "inspect node",
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("need to specify node name")
			}

			node := args[0]

			client, err := getHttpClient(options.Port)
			if err != nil {
				return err
			}
			return doGet(client, node)
		},
	}
}

func getHttpClient(port string) (*http.Client, error) {
	client := &http.Client{}

	// If we can't parse the port as an int, assume it's a unix socket
	if _, err := strconv.Atoi(port); err != nil {
		client.Transport = &http.Transport{
			Dial: func(proto, addr string) (conn net.Conn, err error) {
				return net.Dial("unix", port)
			},
		}
	}

	return client, nil
}

func doList(client *http.Client) error {
	resp, err := client.Get("http://h/machines/json")
	if err != nil {
		return err
	}

	buff, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error:%v, message=%v", resp.StatusCode, buff)
	}

	list := []string{}

	err = json.Unmarshal(buff, &list)
	if err != nil {
		return err
	}

	for _, l := range list {
		fmt.Println(l)
	}
	return nil
}

func doGet(client *http.Client, node string) error {
	resp, err := client.Get(fmt.Sprintf("http://h/machines/%s/json", node))
	if err != nil {
		return err
	}

	buff, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error:%v, message=%v", resp.StatusCode, buff)
	}

	record := new(storage.MachineRecord)

	err = json.Unmarshal(buff, record)
	if err != nil {
		return err
	}

	buff, err = json.MarshalIndent(record, "  ", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(buff))
	return nil
}

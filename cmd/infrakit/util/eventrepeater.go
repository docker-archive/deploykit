package util

import (
	"bytes"
	"context"
	"fmt"
	log "github.com/Sirupsen/logrus"
	eventrepeater "github.com/docker/infrakit/pkg/application/eventrepeater"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

func eventrepeaterCommand(plugins func() discovery.Plugins) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event-repeater",
		Short: "Event Repeater service",
	}
	cmd.AddCommand(errunCommand(plugins), ermanageCommand(plugins))
	return cmd
}

func errunCommand(plugins func() discovery.Plugins) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run Event Repeater service",
	}

	name := cmd.Flags().String("name", "app-event-repeater", "Application name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	source := cmd.Flags().String("source", "event-plugin", "Event sourve address.")
	sink := cmd.Flags().String("sink", "localhost:1883", "Event sink address. default: localhost:1883")
	sinkProtocol := cmd.Flags().String("sinkprotocol", "mqtt", "Event sink protocol. Now only mqtt and stderr is implemented.")
	allowall := cmd.Flags().Bool("allowall", false, "Allow all event from source and repeat the event to sink as same topic name. default: false")
	listen := cmd.Flags().String("listen", "", "Application listen host:port")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		cli.SetLogLevel(*logLevel)
		dir := local.Dir()
		os.MkdirAll(dir, 0700)
		discoverPath := path.Join(dir, *name)
		if *listen != "" {
			discoverPath += ".listen"
		}
		pidPath := path.Join(dir, *name+".pid")
		e := eventrepeater.NewEventRepeater(*source, *sink, *sinkProtocol, *allowall)
		s, err := e.Serve(discoverPath, *listen)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(pidPath, []byte(fmt.Sprintf("%v", os.Getpid())), 0644)
		if err != nil {
			return err
		}
		log.Infoln("PID file at", pidPath)
		if s != nil {
			s.AwaitStopped()
		}
		// clean up
		os.Remove(pidPath)
		log.Infoln("Removed PID file at", pidPath)
		os.Remove(discoverPath)
		log.Infoln("Removed discover file at", discoverPath)

		return nil
	}
	return cmd
}

func ermanageCommand(plugins func() discovery.Plugins) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manage",
		Short: "management Event Repeater service",
	}
	name := cmd.PersistentFlags().String("name", "", "Name of plugin")
	path := cmd.PersistentFlags().String("path", "/events", "URL path of events default /events")
	if !strings.HasPrefix(*path, "/") {
		fmt.Printf("Path must start from \"/\" : %s ", *path)
		return nil
	}
	var addr string
	var protocol string
	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if err := cli.EnsurePersistentPreRunE(c); err != nil {
			return err
		}
		endpoint, err := plugins().Find(plugin.Name(*name))
		if err != nil {
			return err
		}
		addr = endpoint.Address
		protocol = endpoint.Protocol
		return nil
	}

	value := ""

	send := func(method string, body string) error {
		switch protocol {
		case "tcp":
			url := strings.Replace(addr, "tcp:", "http:", 1)
			req, err := http.NewRequest(
				method,
				url+*path,
				bytes.NewBuffer([]byte(body)),
			)
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: time.Duration(10) * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			respbody, _ := ioutil.ReadAll(resp.Body)
			logger.Info("Send Request", "URL", url+*path, "Request Body", body, "Respence Status", resp.Status, "Respence Body", string(respbody))
			defer resp.Body.Close()
		case "unix":
			req, err := http.NewRequest(
				method,
				"http://unix"+*path,
				bytes.NewBuffer([]byte(body)),
			)
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			client := http.Client{
				Transport: &http.Transport{
					DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
						return net.Dial("unix", addr)
					},
				},
				Timeout: time.Duration(10) * time.Second,
			}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			respbody, _ := ioutil.ReadAll(resp.Body)
			logger.Info("Send Request", "URL", addr+*path, "Request Body", body, "Respence Status", resp.Status, "Respence Body", string(respbody))
			defer resp.Body.Close()
		}
		return nil
	}
	post := &cobra.Command{
		Use:   "post",
		Short: "Post request to event repeater.",
		RunE: func(c *cobra.Command, args []string) error {
			err := send("POST", value)
			if err != nil {
				return err
			}
			return nil
		},
	}
	post.Flags().StringVar(&value, "value", value, "update value")

	delete := &cobra.Command{
		Use:   "delete",
		Short: "Delete request to event repeater.",
		RunE: func(c *cobra.Command, args []string) error {
			err := send("DELETE", value)
			if err != nil {
				return err
			}

			return nil
		},
	}
	delete.Flags().StringVar(&value, "value", value, "update value")

	put := &cobra.Command{
		Use:   "put",
		Short: "Put request to event repeater.",
		RunE: func(c *cobra.Command, args []string) error {
			err := send("PUT", value)
			if err != nil {
				return err
			}

			return nil
		},
	}
	put.Flags().StringVar(&value, "value", value, "update value")

	get := &cobra.Command{
		Use:   "get",
		Short: "Get request to event repeater.",
		RunE: func(c *cobra.Command, args []string) error {
			err := send("GET", value)
			if err != nil {
				return err
			}
			return nil
		},
	}

	cmd.AddCommand(post)
	cmd.AddCommand(delete)
	cmd.AddCommand(get)
	cmd.AddCommand(put)

	return cmd

}

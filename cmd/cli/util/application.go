package util

import (
	"bytes"
	"context"
	"fmt"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

// Command is the entry point of the module
func applicationCommand(plugins func() discovery.Plugins) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "application",
		Short: "Access application plugins",
	}
	name := cmd.PersistentFlags().String("name", "", "Name of plugin")
	path := cmd.PersistentFlags().String("path", "/", "URL path of resource e.g. /resources/resourceID/")
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
		Short: "Post request to application.",
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
		Short: "Delete request to application.",
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
		Short: "Put request to application.",
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
		Short: "Get request to application.",
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

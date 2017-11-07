package loadbalancer

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/spf13/cobra"
)

// Routes returns the describe command
func Routes(name string, services *cli.Services) *cobra.Command {
	routes := &cobra.Command{
		Use:   "routes",
		Short: "Loadbalancer routes",
	}

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List loadbalancer routes",
	}

	publish := &cobra.Command{
		Use:   "add <frontend_port> <backend_port>",
		Short: "Publish a loadbalancer route",
	}
	unpublish := &cobra.Command{
		Use:   "rm <frontend_port>, ....",
		Short: "Unpublish a loadbalancer route",
	}

	routes.AddCommand(ls, publish, unpublish)

	protocol := publish.Flags().String("protocol", "tcp", "Protocol: http|https|tcp|udp|ssl")
	loadbalancerProtocol := publish.Flags().String("loadbalancerprotocol", "tcp", "Protocol: http|https|tcp|udp|ssl")

	cert := publish.Flags().String("cert", "", "Certificate")

	publish.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 2 {
			cmd.Usage()
			os.Exit(1)
		}

		frontendPort, err := strconv.Atoi(args[0])
		if err != nil {
			return err
		}

		backendPort, err := strconv.Atoi(args[1])
		if err != nil {
			return err
		}

		l4, err := services.Scope.L4(name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(l4, "L4 not found", "name", name)

		route := loadbalancer.Route{
			LoadBalancerPort:     frontendPort,
			LoadBalancerProtocol: loadbalancer.Protocol(*loadbalancerProtocol),
			Port:                 backendPort,
			Protocol:             loadbalancer.Protocol(*protocol),
		}
		if *cert != "" {
			copy := *cert
			route.Certificate = &copy
		}
		res, err := l4.Publish(route)
		fmt.Println(res)
		return err
	}

	unpublish.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		l4, err := services.Scope.L4(name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(l4, "L4 not found", "name", name)

		targets := []int{}
		for _, a := range args {
			v, err := strconv.Atoi(a)
			if err != nil {
				return err
			}
			targets = append(targets, v)
		}

		for _, t := range targets {
			res, err := l4.Unpublish(t)
			if err != nil {
				return err
			}
			fmt.Println(res)
		}
		return err
	}

	ls.Flags().AddFlagSet(services.OutputFlags)
	ls.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			cmd.Usage()
			os.Exit(1)
		}

		l4, err := services.Scope.L4(name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(l4, "L4 not found", "name", name)

		list, err := l4.Routes()
		if err != nil {
			return err
		}

		return services.Output(os.Stdout, list,
			func(w io.Writer, v interface{}) error {

				fmt.Printf("%-15v  %-20v   %-15v   %-20v    %-20v\n", "FRONTEND PORT", "FRONTEND PROTOCOL", "BACKEND PORT", "BACKEND PROTOCOL", "CERT")
				for _, r := range list {
					cert := ""
					if r.Certificate != nil {
						cert = *r.Certificate
					}
					fmt.Printf("%-15v  %-20v   %-15v   %-20v    %-20v\n", r.LoadBalancerPort, r.LoadBalancerProtocol, r.Port, r.Protocol, cert)
				}
				return nil
			})
	}
	return routes
}

package main

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/docker/infrakit/pkg/cli"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin/instance/hyperkit"
	"github.com/docker/infrakit/pkg/plugin/metadata"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	metadata_plugin "github.com/docker/infrakit/pkg/rpc/metadata"
	instance_spi "github.com/docker/infrakit/pkg/spi/instance"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "instance/hyperkit")

func init() {
	logutil.Configure(&logutil.ProdDefaults)
}

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "HyperKit instance plugin",
	}

	defaultVMDir := filepath.Join(getHome(), ".infrakit/hyperkit-vms")

	name := cmd.Flags().String("name", "instance-hyperkit", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")

	vmDir := cmd.Flags().String("vm-dir", defaultVMDir, "Directory where to store VM state")
	hyperkitCmd := cmd.Flags().String("hyperkit", "", "Path to HyperKit executable")
	vpnkitSock := cmd.Flags().String("vpnkit-sock", "auto", "Path to VPNKit UNIX domain socket")
	listen := cmd.Flags().String("listen", "localhost:24865", "Listens on port")

	cmd.RunE = func(c *cobra.Command, args []string) error {

		os.MkdirAll(*vmDir, os.ModePerm)

		if _, err := exec.LookPath(*hyperkitCmd); err != nil {
			return err
		}

		cli.SetLogLevel(*logLevel)
		cli.RunListener(*listen, *name,
			instance_plugin.PluginServer(hyperkit.NewPlugin(*vmDir, *hyperkitCmd, *vpnkitSock)),
			metadata_plugin.PluginServer(metadata.NewPluginFromData(
				map[string]interface{}{
					"version":    cli.Version,
					"revision":   cli.Revision,
					"implements": instance_spi.InterfaceSpec,
				},
			)),
		)
		return nil
	}

	cmd.AddCommand(cli.VersionCommand())

	if err := cmd.Execute(); err != nil {
		log.Crit("Error", "err", err)
		os.Exit(1)
	}
}

func getHome() string {
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return os.Getenv("HOME")
}

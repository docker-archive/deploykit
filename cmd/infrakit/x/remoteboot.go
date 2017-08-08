package x

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/docker/infrakit/pkg/x/remoteboot"
	"github.com/spf13/cobra"
)

// Static URL for retrieving the bootloader
const iPXEURL = "https://boot.ipxe.org/undionly.kpxe"

func remoteBootCommand() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "remoteboot",
		Short: "Used to remotely boot OS instances",
	}

	addressDHCP := cmd.Flags().String("addressDHCP", "", "Address to advertise leases from, ideally will be the IP address of --adapter")
	addressHTTP := cmd.Flags().String("addressHTTP", "", "Address of HTTP to use, if blank will default to [dhcpAddress]")
	addressTFTP := cmd.Flags().String("addressTFTP", "", "Address of TFTP to use, if blank will default to [dhcpAddress]")

	enableDHCP := cmd.Flags().Bool("enableDHCP", false, "Enable the DCHP Server")
	enableTFTP := cmd.Flags().Bool("enableTFTP", false, "Enable the TFTP Server")
	enableHTTP := cmd.Flags().Bool("enableHTTP", false, "Enable the HTTP Server")

	adapter := cmd.Flags().String("adapter", "", "Name of adapter to use e.g eth0, en0")
	iPXEPath := cmd.Flags().String("iPXEPath", "undionly.kpxe", "Path to an iPXE bootloader")
	gateway := cmd.Flags().String("gateway", "", "Address of Gateway to use, if blank will default to [dhcpAddress]")
	dns := cmd.Flags().String("dns", "", "Address of DNS to use, if blank will default to [dhcpAddress]")
	leasecount := cmd.Flags().Int("leasecount", 20, "Amount of leases to advertise")
	startAddress := cmd.Flags().String("startAddress", "", "Start advertised address [REQUIRED]")

	var pulliPXE = &cobra.Command{
		Use:   "pulliPXE",
		Short: "Attempts to download iPXE to the current working directory",
		Run: func(cmd *cobra.Command, args []string) {
			pullPXEBooter()
		},
	}
	cmd.AddCommand(pulliPXE)

	cmd.RunE = func(c *cobra.Command, args []string) error {

		if *adapter == "" {
			cmd.Usage()
			log.Crit("Flag --adapter is blank")
			return nil
		}
		if *startAddress == "" {
			cmd.Usage()
			log.Crit("Flag --startAddress is blank")
			return nil
		}
		remote, err := remoteboot.NewRemoteBoot(*adapter,
			*addressDHCP,
			*addressHTTP,
			*addressTFTP,
			*iPXEPath,
			*gateway,
			*dns,
			*leasecount,
			*startAddress)

		if err != nil {
			log.Crit("%v", err)
		}

		remote.StartServices(*enableDHCP, *enableTFTP, *enableHTTP)
		return nil
	}
	return cmd
}

// PullPXEBooter - This will attempt to download the iPXE bootloader
func pullPXEBooter() {
	fmt.Printf("Beginning of iPXE download... ")

	// Create the file
	out, err := os.Create("undionly.kpxe")
	if err != nil {
		fmt.Printf("%v", err)
		os.Exit(-1)
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(iPXEURL)
	if err != nil {
		fmt.Printf("%v", err)
		os.Exit(-1)
	}
	defer resp.Body.Close()

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Printf("%v", err)
		os.Exit(-1)
	}
	fmt.Printf("Completed\n")
	os.Exit(0)
}

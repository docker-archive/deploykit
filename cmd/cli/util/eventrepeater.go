package util

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	eventrepeater "github.com/docker/infrakit/pkg/application/eventrepeater"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"path"
)

func eventrepeaterCommand(plugins func() discovery.Plugins) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event-repeater",
		Short: "Event Repeater service",
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

package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/controller"
	"github.com/docker/libmachete/controller/watcher"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"time"
)

func getURL(url string) ([]byte, error) {
	client := &http.Client{}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusOK {
		return ioutil.ReadAll(resp.Body)
	}
	return nil, fmt.Errorf("http error=%d", resp.StatusCode)
}

func watchURL() *cobra.Command {

	pollingInterval := time.Duration(1 * time.Second)
	watch := &cobra.Command{
		Use:   "url",
		Short: "Watches resource represented by a URL",
		RunE: func(_ *cobra.Command, args []string) error {

			if len(args) == 0 {
				return fmt.Errorf("no url specified")
			}

			url := args[0]

			w := watcher.New(
				func(change chan<- []byte, stop <-chan struct{}) {

					tick := time.Tick(pollingInterval)

					for {
						select {
						case <-stop:
							log.Infoln("Watcher stopped.")
							return
						case <-tick:
							data, err := getURL(url)

							log.Debugln("fetched", url, "len=", len(data), "err=", err)

							if err == nil {
								change <- data
							} else {
								log.Warningln("Cannot fetch resource at", url)
							}
						}
					}
				},
				func(buff []byte) {
					log.Infoln("Change detected. Restarting controllers")
					names, err := POC2ControllerNamesFromSWIM(buff)
					if err != nil {
						log.Warningln("Cannot parse input.", err)
						return
					}

					// get the configs for all the controllers -- map of controller to config
					changeSet := map[*controller.Controller]interface{}{}

					for _, name := range names {

						controller := registry.GetControllerByName(name)
						if controller == nil {
							log.Warningln("No controller found for name=", name, "Do nothing.")
							return
						}

						config, err := POC2ConfigFromSWIM(buff, controller.Info.Namespace)
						if err != nil {
							log.Warningln("Error while locating configuration of controller", controller.Info.Name)
							return
						}

						// config can be null...
						// TODO(chungers) -- think about this case... no config we assume no change / no need to restart.

						if config != nil {
							changeSet[controller] = config
						}
					}

					// Now run the changes
					// Note there's no specific ordering.  If we are smart we could build dependency into the swim like CFN ;)

					for controller := range changeSet {

						log.Infoln("Restarting controller", controller.Info.Name)
						restart := watcher.Restart(docker, controller.Info.Image)

						err := restart.Run()
						if err != nil {
							log.Warningln("Unable to restart controller", controller.Info.Name)

							// TODO(chungers) -- Do we fail here???  If not all controllers can come back up
							// then we cannot to any updates and cannot maintain state either...

							continue
						}
					}

					// At this point all controllers are running in a latent state.
					// reconfigure all the controllers

					for controller, config := range changeSet {

						log.Infoln("Configuring controller", controller.Info.Name, "with config", config)

						err := controller.Client.Call(controller.Info.DriverType+".Start", config)
						if err != nil {
							// BAD NEWS -- here we cannot get consistency now since one of the controller cannot
							// be updated.  Should we punt -- roll back is impossible at the moment
							log.Warningln("Failed to reconfigure controller", controller.Info.Name, "err=", err)
						}

					}
				})

			<-w.Run()

			return nil
		},
	}

	watch.Flags().DurationVar(&pollingInterval, "interval", pollingInterval, "polling interval")
	return watch
}

package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/controller/watcher"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"time"
)

func getUrl(url string) ([]byte, error) {
	client := &http.Client{}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusOK {
		return ioutil.ReadAll(resp.Body)
	} else {
		return nil, fmt.Errorf("http error=%d", resp.StatusCode)
	}
}

func watchUrl() *cobra.Command {

	pollingInterval := time.Duration(1 * time.Second)
	watch := &cobra.Command{
		Use:   "url",
		Short: "Watches resource represented by a URL",
		RunE: func(_ *cobra.Command, args []string) error {

			if len(args) == 0 {
				return fmt.Errorf("no url specified.")
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
							data, err := getUrl(url)

							log.Debugln("fetched", url, "len=", len(data), "err=", err)

							if err == nil {
								change <- data
							} else {
								log.Warningln("Cannot fetch resource at", url)
							}
						}
					}
				},
				RestartControllers)

			<-w.Run()

			return nil
		},
	}

	watch.Flags().DurationVar(&pollingInterval, "interval", pollingInterval, "polling interval")
	return watch
}

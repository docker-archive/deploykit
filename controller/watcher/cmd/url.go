package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
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

func watchURL(backend *backend) *cobra.Command {

	pollingInterval := time.Duration(1 * time.Second)
	watch := &cobra.Command{
		Use:   "url",
		Short: "Watches resource represented by a URL",
		RunE: func(_ *cobra.Command, args []string) error {

			if len(args) == 0 {
				return fmt.Errorf("no url specified")
			}

			url := args[0]

			// set up the watch function for the backend watcher
			backend.watcher.SetWatch(
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
				})
			return nil
		},
	}

	watch.Flags().DurationVar(&pollingInterval, "interval", pollingInterval, "polling interval")
	return watch
}

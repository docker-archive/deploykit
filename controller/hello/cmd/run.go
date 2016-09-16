package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/controller/hello"
	"github.com/spf13/cobra"
)

func runCommand(backend *backend) *cobra.Command {

	run := &cobra.Command{
		Use:   "run",
		Short: "Runs the server",
		RunE: func(_ *cobra.Command, args []string) error {

			if backend.docker == nil {
				return fmt.Errorf("err-no-docker")
			}

			data := make(chan []byte)
			backend.data = data
			backend.service = hello.New(Name, backend.options, data, backend.docker)

			go backend.service.Run()
			log.Infoln(Name, "started")
			return nil
		},
	}
	return run
}

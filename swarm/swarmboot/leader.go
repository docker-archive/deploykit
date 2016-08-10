package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types/swarm"
	"golang.org/x/net/context"
)

func initializeSwarm(localDocker *client.Client) error {
	_, err := localDocker.SwarmInit(context.Background(), swarm.InitRequest{ListenAddr: "0.0.0.0"})
	if err != nil {
		return err
	}
	log.Info("Initialized swarm")
	return nil
}

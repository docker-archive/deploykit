package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types/swarm"
	"golang.org/x/net/context"
	"time"
)

const (
	joinRetries     = 5
	joinRetryWait   = 10 * time.Second
)

func joinSwarm(localDocker *client.Client, joinIP string, joinToken string) error {
	join := func() bool {
		err := localDocker.SwarmJoin(context.Background(), swarm.JoinRequest{
			ListenAddr:  "0.0.0.0",
			RemoteAddrs: []string{joinIP},
			JoinToken:   joinToken,
		})
		if err != nil {
			log.Warnf("Failed to join swarm: %s", err)
			return false
		}
		return true
	}

	joined := doUntilSuccess(join, joinRetries, joinRetryWait)
	if !joined {
		return fmt.Errorf("Failed to join boot leader %s", joinIP)
	}

	log.Info("Successfully joined")
	return nil
}

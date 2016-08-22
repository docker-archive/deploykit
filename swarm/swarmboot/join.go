package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types/swarm"
	"golang.org/x/net/context"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	tokenServerPort = "8889"
	tokenRetries    = 20
	tokenRetryWait  = 30 * time.Second
	joinRetries     = 5
	joinRetryWait   = 10 * time.Second
)

// TODO(wfarner): Consider accepting a list of IP addresses, and have this routine fall back to different tokenservers
// and engines for better redundancy.

func joinSwarm(localDocker *client.Client, joinIP string, manager bool) error {
	var joinToken string

	var tokenEndpoint string
	if manager {
		tokenEndpoint = "manager"
	} else {
		tokenEndpoint = "worker"
	}

	fetchToken := func() bool {
		resp, err := http.Get(fmt.Sprintf("http://%s:%s/token/%s", joinIP, tokenServerPort, tokenEndpoint))
		if err != nil {
			log.Warnf("Failed to retrieve join token: %s", err)
			return false
		}

		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Warnf("Failed to retrieve join token: %s", err)
			return false
		}

		joinToken = string(bodyBytes)
		return true
	}

	foundToken := doUntilSuccess(fetchToken, tokenRetries, tokenRetryWait)
	if !foundToken {
		return fmt.Errorf("Failed to find boot leader %s", joinIP)
	}
	log.Infof("Discovered manager join token %s", joinToken)

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

package group

import (
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/spi/instance"
	"reflect"
	"time"
)

type quorum struct {
	scaled       Scaled
	IPs          []string
	pollInterval time.Duration
	stop         chan bool
}

// NewQuorum creates a supervisor for a group of instances operating in a quorum.
func NewQuorum(scaled Scaled, IPs []string, pollInterval time.Duration) Supervisor {
	return &quorum{
		scaled:       scaled,
		IPs:          IPs,
		pollInterval: pollInterval,
		stop:         make(chan bool),
	}
}

func (q *quorum) PlanUpdate(scaled Scaled, settings groupSettings, newSettings groupSettings) (updatePlan, error) {
	if settings.config.Size != newSettings.config.Size {
		return nil, errors.New("A quorum group cannot be resized")
	}

	if !reflect.DeepEqual(settings.config.IPs, newSettings.config.IPs) {
		return nil, errors.New("IP address changes to a quorum is not currently supported")
	}

	return &rollingupdate{
		desc:       fmt.Sprintf("Performs a rolling update on %d instances", len(settings.config.IPs)),
		scaled:     scaled,
		updatingTo: newSettings,
		stop:       make(chan bool),
	}, nil
}

func (q *quorum) Stop() {
	close(q.stop)
}

func (q *quorum) Run() {
	ticker := time.NewTicker(q.pollInterval)

	for {
		select {
		case <-ticker.C:
			q.converge()

		case <-q.stop:
			ticker.Stop()
			return
		}
	}
}

func (q *quorum) converge() {
	descriptions, err := q.scaled.List()
	if err != nil {
		log.Infof("Failed to check group: %s", err)
		return
	}

	log.Debugf("Found existing instances: %v", descriptions)

	unknownIPs := []instance.Description{}
	for _, description := range descriptions {
		matched := false
		for _, expectedIP := range q.IPs {
			if expectedIP == description.PrivateIPAddress {
				matched = true
			}
		}
		if !matched {
			unknownIPs = append(unknownIPs, description)
		}
	}

	for _, unknownInstance := range unknownIPs {
		log.Warnf("Destroying instances with unknown IP address: %+v", unknownInstance)
		q.scaled.Destroy(unknownInstance.ID)
	}

	missingIPs := []string{}
	for _, expectedIP := range q.IPs {
		matched := false
		for _, description := range descriptions {
			if expectedIP == description.PrivateIPAddress {
				matched = true
			}
		}
		if !matched {
			missingIPs = append(missingIPs, expectedIP)
		}
	}

	for _, missingIP := range missingIPs {
		log.Infof("IP %s is missing, provisioning new instance", missingIP)
		ip := missingIP
		q.scaled.CreateOne(&ip)
	}
}

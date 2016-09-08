package quorum

import (
	"bytes"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/controller/util"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
	"text/template"
	"time"
)

type quorum struct {
	pollInterval      time.Duration
	provisioner       instance.Plugin
	provisionTemplate *template.Template
	group             group.ID
	ipAddresses       []string
	stop              chan bool
}

// NewQuorum creates a RunStop that manages a quorum on a provisioner, attempting to maintain a fixed count.
func NewQuorum(
	pollInterval time.Duration,
	provisioner instance.Plugin,
	provisionTemplate string,
	ipAddresses []string) (util.RunStop, error) {

	group, _, err := groupAndCountFromRequest(provisionTemplate)
	if err != nil {
		return nil, err
	}

	parsed, err := template.New("test").Parse(provisionTemplate)
	if err != nil {
		return nil, err
	}

	return &quorum{
		pollInterval:      pollInterval,
		provisioner:       provisioner,
		provisionTemplate: parsed,
		group:             *group,
		ipAddresses:       ipAddresses,
		stop:              make(chan bool),
	}, nil
}

func (q *quorum) checkState() {
	log.Debugf("Checking instance count for group %s", q.group)
	descriptions, err := q.provisioner.DescribeInstances(q.group)
	if err != nil {
		log.Infof("Failed to check count of %s: %s", q.group, err)
		return
	}

	log.Debugf("Found existing instances: %v", descriptions)

	unknownIPs := []instance.Description{}
	for _, description := range descriptions {
		matched := false
		for _, expectedIP := range q.ipAddresses {
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
		err = q.provisioner.Destroy(unknownInstance.ID)
		if err != nil {
			log.Errorf("Failed to destroy instance: %v", err)
		}
	}

	missingIPs := []string{}
	for _, expectedIP := range q.ipAddresses {
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
		err := ProvisionManager(q.provisioner, q.group, q.provisionTemplate, missingIP)
		if err != nil {
			log.Error(err)
			continue
		}
	}
}

// ProvisionManager creates a single manager instance, replacing the IP address wildcard with the provided IP.
func ProvisionManager(provisioner instance.Plugin, gid group.ID, provisionTemplate *template.Template, ip string) error {
	buffer := bytes.Buffer{}
	err := provisionTemplate.Execute(&buffer, map[string]string{"IP": ip})
	if err != nil {
		return fmt.Errorf("Failed to create provision request: %s", err)
	}

	volume := instance.VolumeID(ip)
	id, err := provisioner.Provision(gid, buffer.String(), &volume)
	if err != nil {
		return fmt.Errorf("Failed to provision: %s", err)
	}

	log.Infof("Provisioned instance %s with IP %s", *id, ip)
	return nil
}

func (q *quorum) Run() {
	ticker := time.NewTicker(q.pollInterval)

	for {
		select {
		case <-ticker.C:
			q.checkState()
		case <-q.stop:
			ticker.Stop()
			return
		}
	}
}

func (q *quorum) Stop() {
	q.stop <- true
}

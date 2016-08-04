package quorum

import (
	"bytes"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/controller/util"
	"github.com/docker/libmachete/spi/instance"
	"text/template"
	"time"
)

type quorum struct {
	pollInterval      time.Duration
	provisioner       instance.Provisioner
	provisionTemplate *template.Template
	group             instance.GroupID
	ipAddresses       []string
	stop              chan bool
}

// NewQuorum creates a RunStop that manages a quorum on a provisioner, attempting to maintain a fixed count.
func NewQuorum(
	pollInterval time.Duration,
	provisioner instance.Provisioner,
	provisionTemplate string,
	ipAddresses []string) (util.RunStop, error) {

	group, err := util.GroupFromRequest(provisionTemplate)
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

		buffer := bytes.Buffer{}
		err := q.provisionTemplate.Execute(&buffer, templateInput{IP: missingIP})
		if err != nil {
			log.Errorf("Failed to create provision request: %s", err)
			continue
		}

		id, err := q.provisioner.Provision(buffer.String())
		if err != nil {
			log.Errorf("Failed to provision: %s", err)
			continue
		}

		log.Infof("Provisioned instance %s with IP %s", *id, missingIP)
	}
}

type templateInput struct {
	IP string
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

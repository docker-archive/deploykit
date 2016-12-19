package gcp

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"cloud.google.com/go/compute/metadata"
)

const firewallRuleSuffix = "-allow-lb"

// findLBFirewallRule finds the name of the firewall rule used for the load
// balancer. This uses the instance metadata server.
func findLBFirewallRule() (string, error) {
	stack, err := findStack()
	if err != nil {
		return "", err
	}

	firewallRule := stack + firewallRuleSuffix
	log.Debugln("Found load balancer firewall rule", stack)
	return firewallRule, err
}

// findStack finds the name of deployment stack that the instance running this
// code is deployed in. This uses the instance metadata server.
func findStack() (string, error) {
	network, err := findNetwork()
	if err != nil {
		return "", err
	}

	stack := network[0 : len(network)-len("-network")]
	log.Debugln("Found stack", stack)
	return stack, err
}

// findNetwork finds the name of network stack that the instance running this
// code is connected to. This uses the instance metadata server.
func findNetwork() (string, error) {
	network, err := metadata.Get("instance/network-interfaces/0/network")
	if err != nil {
		return "", err
	}

	network = last(network)
	if !strings.HasSuffix(network, "-network") {
		return "", fmt.Errorf("Invalid network name: %s", network)
	}

	log.Debugln("Found network", network)
	return network, err
}

// findProject finds the current project. This uses the instance metadata
// server.
func findProject() (string, error) {
	project, err := metadata.ProjectID()
	if err != nil {
		return "", err
	}

	log.Debugln("Found project", project)
	return project, err
}

// last extracts the last component of a path:
// a/b/c -> c
func last(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}

// +build providers

package main

import (
	// By default we disable building of libvirt and ucs
	_ "github.com/docker/infrakit/pkg/run/v0/aws"
	_ "github.com/docker/infrakit/pkg/run/v0/digitalocean"
	_ "github.com/docker/infrakit/pkg/run/v0/docker"
	_ "github.com/docker/infrakit/pkg/run/v0/google"
	_ "github.com/docker/infrakit/pkg/run/v0/hyperkit"
	_ "github.com/docker/infrakit/pkg/run/v0/ibmcloud"
	_ "github.com/docker/infrakit/pkg/run/v0/kubernetes"
	_ "github.com/docker/infrakit/pkg/run/v0/maas"
	_ "github.com/docker/infrakit/pkg/run/v0/oneview"
	_ "github.com/docker/infrakit/pkg/run/v0/oracle"
	_ "github.com/docker/infrakit/pkg/run/v0/packet"
	_ "github.com/docker/infrakit/pkg/run/v0/rackhd"
	_ "github.com/docker/infrakit/pkg/run/v0/terraform"
	_ "github.com/docker/infrakit/pkg/run/v0/vagrant"
	_ "github.com/docker/infrakit/pkg/run/v0/vsphere"
)

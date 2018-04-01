// +build builtin

package main

import (
	_ "github.com/docker/infrakit/pkg/run/v0/combo"
	_ "github.com/docker/infrakit/pkg/run/v0/enrollment"
	_ "github.com/docker/infrakit/pkg/run/v0/file"
	_ "github.com/docker/infrakit/pkg/run/v0/gc"
	_ "github.com/docker/infrakit/pkg/run/v0/group"
	_ "github.com/docker/infrakit/pkg/run/v0/image"
	_ "github.com/docker/infrakit/pkg/run/v0/ingress"
	_ "github.com/docker/infrakit/pkg/run/v0/inventory"
	_ "github.com/docker/infrakit/pkg/run/v0/manager"
	_ "github.com/docker/infrakit/pkg/run/v0/pool"
	_ "github.com/docker/infrakit/pkg/run/v0/resource"
	_ "github.com/docker/infrakit/pkg/run/v0/selector"
	_ "github.com/docker/infrakit/pkg/run/v0/simulator"
	_ "github.com/docker/infrakit/pkg/run/v0/swarm"
	_ "github.com/docker/infrakit/pkg/run/v0/tailer"
	_ "github.com/docker/infrakit/pkg/run/v0/time"
	_ "github.com/docker/infrakit/pkg/run/v0/vanilla"
	_ "github.com/docker/infrakit/pkg/run/v0/vars"
)

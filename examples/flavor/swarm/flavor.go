package main

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	docker_types "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/docker"
	"golang.org/x/net/context"
)

const (
	ebsAttachment string = "ebs"
)

// Spec is the value passed in the `Properties` field of configs
type Spec struct {

	// Attachments indicate the devices that are to be attached to the instance
	Attachments map[instance.LogicalID][]instance.Attachment

	// InitScriptTemplateURL overrides the template specified when the plugin started up.
	InitScriptTemplateURL string

	// SwarmJoinIP is the IP for managers and workers to join
	SwarmJoinIP string

	// Docker holds the connection params to the Docker engine for join tokens, etc.
	Docker ConnectInfo
}

// ConnectInfo holds the connection parameters for connecting to a Docker engine to get join tokens, etc.
type ConnectInfo struct {
	Host string
	TLS  *tlsconfig.Options
}

// DockerClient checks the validity of input spec and connects to Docker engine
func DockerClient(spec Spec) (client.APIClient, error) {
	if spec.Docker.Host == "" && spec.Docker.TLS == nil {
		return nil, fmt.Errorf("no docker connect info")
	}
	tls := spec.Docker.TLS
	if tls == nil {
		tls = &tlsconfig.Options{}
	}

	return docker.NewDockerClient(spec.Docker.Host, tls)
}

func validateIDsAndAttachments(logicalIDs []instance.LogicalID,
	attachments map[instance.LogicalID][]instance.Attachment) error {

	// Each attachment association must be represented by a logical ID.
	idsMap := map[instance.LogicalID]bool{}
	for _, id := range logicalIDs {
		if _, exists := idsMap[id]; exists {
			return fmt.Errorf("LogicalID %v specified more than once", id)
		}

		idsMap[id] = true
	}
	for id := range attachments {
		if _, exists := idsMap[id]; !exists {
			return fmt.Errorf("LogicalID %v used for an attachment but is not in group LogicalIDs", id)
		}
	}

	// Only EBS attachments are supported.
	for _, atts := range attachments {
		for _, attachment := range atts {
			if attachment.Type == "" {
				return fmt.Errorf(
					"Attachment Type %s must be specified for '%s'",
					ebsAttachment,
					attachment.ID)
			}

			if attachment.Type != ebsAttachment {
				return fmt.Errorf(
					"Invalid attachment Type '%s', only %s is supported",
					attachment.Type,
					ebsAttachment)
			}
		}
	}

	// Each attachment may only be used once.
	allAttachmentIDs := map[string]bool{}
	for _, atts := range attachments {
		for _, attachment := range atts {
			if _, exists := allAttachmentIDs[attachment.ID]; exists {
				return fmt.Errorf("Attachment %v specified more than once", attachment.ID)
			}
			allAttachmentIDs[attachment.ID] = true
		}
	}

	return nil
}

func swarmState(docker client.APIClient) (status *swarm.Swarm, node *swarm.Node, err error) {
	ctx := context.Background()
	info, err := docker.Info(ctx)
	if err != nil {
		log.Warningln("Err docker info:", err)
		status = nil
		node = nil
		return
	}
	n, _, err := docker.NodeInspectWithRaw(ctx, info.Swarm.NodeID)
	if err != nil {
		log.Warningln("Err node inspect:", err)
		return
	}

	node = &n

	s, err := docker.SwarmInspect(ctx)
	if err != nil {
		log.Warningln("Err swarm inspect:", err)
		return
	}
	status = &s
	return
}

type templateContext struct {
	flavorSpec   Spec
	instanceSpec instance.Spec
	allocation   group_types.AllocationMethod
	swarmStatus  *swarm.Swarm
	nodeInfo     *swarm.Node
	link         types.Link
	retries      int
	poll         time.Duration
}

// Funcs implements the template.Context interface
func (c *templateContext) Funcs() []template.Function {
	return []template.Function{
		{
			Name:        "SWARM_CONNECT_RETRIES",
			Description: "Connect to the swarm manager",
			Func: func(retries int, wait string) interface{} {
				c.retries = retries
				poll, err := time.ParseDuration(wait)
				if err != nil {
					poll = 1 * time.Minute
				}
				c.poll = poll
				return ""
			},
		},
		{
			Name:        "SPEC",
			Description: "The flavor spec as found in Properties field of the config JSON",
			Func: func() interface{} {
				return c.flavorSpec
			},
		},
		{
			Name:        "INSTANCE_LOGICAL_ID",
			Description: "The logical id for the instance being prepared; can be empty if no logical ids are set (cattle).",
			Func: func() string {
				if c.instanceSpec.LogicalID != nil {
					return string(*c.instanceSpec.LogicalID)
				}
				return ""
			},
		},
		{
			Name:        "ALLOCATIONS",
			Description: "The allocations contain fields such as the size of the group or the list of logical ids.",
			Func: func() interface{} {
				return c.allocation
			},
		},
		{
			Name:        "INFRAKIT_LABELS",
			Description: "The label name to use for linking an InfraKit managed resource somewhere else.",
			Func: func() []string {
				return c.link.KVPairs()
			},
		},
		{
			Name:        "SWARM_MANAGER_IP",
			Description: "The label name to use for linking an InfraKit managed resource somewhere else.",
			Func: func() (string, error) {
				if c.nodeInfo == nil {
					return "", fmt.Errorf("cannot prepare: no node info")
				}
				if c.nodeInfo.ManagerStatus == nil {
					return "", fmt.Errorf("cannot prepare: no manager status")
				}
				return c.nodeInfo.ManagerStatus.Addr, nil
			},
		},
		{
			Name:        "SWARM_INITIALIZED",
			Description: "Returns true if the swarm has been initialized.",
			Func: func() bool {
				if c.nodeInfo == nil {
					return false
				}
				return c.nodeInfo.ManagerStatus != nil
			},
		},
		{
			Name:        "SWARM_JOIN_TOKENS",
			Description: "Returns the swarm JoinTokens object, with either .Manager or .Worker fields",
			Func: func() (interface{}, error) {
				if c.swarmStatus == nil {
					return nil, fmt.Errorf("cannot prepare: no swarm status")
				}
				return c.swarmStatus.JoinTokens, nil
			},
		},
		{
			Name:        "SWARM_CLUSTER_ID",
			Description: "Returns the swarm cluster UUID",
			Func: func() (interface{}, error) {
				if c.swarmStatus == nil {
					return nil, fmt.Errorf("cannot prepare: no swarm status")
				}
				return c.swarmStatus.ID, nil
			},
		},
	}
}

// Healthy determines whether an instance is healthy.  This is determined by whether it has successfully joined the
// Swarm.
func healthy(client client.APIClient, inst instance.Description) (flavor.Health, error) {

	link := types.NewLinkFromMap(inst.Tags)
	if !link.Valid() {
		log.Info("Reporting unhealthy for instance without an association tag", inst.ID)
		return flavor.Unhealthy, nil
	}

	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=%s", link.Label(), link.Value()))

	nodes, err := client.NodeList(context.Background(), docker_types.NodeListOptions{Filters: filter})
	if err != nil {
		return flavor.Unknown, err
	}

	switch {
	case len(nodes) == 0:
		// The instance may not yet be joined, so we consider the health unknown.
		return flavor.Unknown, nil

	case len(nodes) == 1:
		return flavor.Healthy, nil

	default:
		log.Warnf("Expected at most one node with label %s, but found %s", link.Value(), nodes)
		return flavor.Healthy, nil
	}
}

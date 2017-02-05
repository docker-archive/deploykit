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

// baseFlavor is the base implementation.  The manager / worker implementations will provide override.
type baseFlavor struct {
	getDockerClient func(Spec) (client.APIClient, error)
	initScript      *template.Template
}

// Funcs implements the template.FunctionExporter interface that allows the RPC server to expose help on the
// functions it exports
func (s *baseFlavor) Funcs() []template.Function {
	return (&templateContext{}).Funcs()
}

// Validate checks the configuration of flavor plugin.
func (s *baseFlavor) Validate(flavorProperties *types.Any, allocation group_types.AllocationMethod) error {
	if flavorProperties == nil {
		return fmt.Errorf("missing config")
	}

	spec := Spec{}
	err := flavorProperties.Decode(&spec)

	if err != nil {
		return err
	}

	if spec.Docker.Host == "" && spec.Docker.TLS == nil {
		return fmt.Errorf("no docker connect info")
	}

	if spec.InitScriptTemplateURL != "" {
		_, err := template.NewTemplate(spec.InitScriptTemplateURL, defaultTemplateOptions)
		if err != nil {
			return err
		}
	}

	if err := validateIDsAndAttachments(allocation.LogicalIDs, spec.Attachments); err != nil {
		return err
	}

	return nil
}

// Healthy determines whether an instance is healthy.  This is determined by whether it has successfully joined the
// Swarm.
func (s *baseFlavor) Healthy(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
	if flavorProperties == nil {
		return flavor.Unknown, fmt.Errorf("missing config")
	}
	spec := Spec{}
	err := flavorProperties.Decode(&spec)
	if err != nil {
		return flavor.Unknown, err
	}
	dockerClient, err := s.getDockerClient(spec)
	if err != nil {
		return flavor.Unknown, err
	}
	return healthy(dockerClient, inst)
}

func (s *baseFlavor) prepare(role string, flavorProperties *types.Any, instanceSpec instance.Spec,
	allocation group_types.AllocationMethod) (instance.Spec, error) {

	spec := Spec{}
	err := flavorProperties.Decode(&spec)
	if err != nil {
		return instanceSpec, err
	}

	initTemplate := s.initScript

	if spec.InitScriptTemplateURL != "" {

		t, err := template.NewTemplate(spec.InitScriptTemplateURL, defaultTemplateOptions)
		if err != nil {
			return instanceSpec, err
		}

		initTemplate = t
		log.Infoln("Using", spec.InitScriptTemplateURL, "for init script template")
	}

	var swarmID, initScript string
	var swarmStatus *swarm.Swarm
	var node *swarm.Node
	var link *types.Link

	for i := 0; ; i++ {
		log.Debugln(role, ">>>", i, "Querying docker swarm")

		dockerClient, err := s.getDockerClient(spec)
		if err != nil {
			log.Warningln("Cannot connect to Docker:", err)
			continue
		}

		swarmStatus, node, err = swarmState(dockerClient)
		if err != nil {
			log.Warningln("Worker prepare:", err)
		}

		swarmID := "?"
		if swarmStatus != nil {
			swarmID = swarmStatus.ID
		}

		link = types.NewLink().WithContext("swarm/" + swarmID + "/" + role)
		context := &templateContext{
			flavorSpec:   spec,
			instanceSpec: instanceSpec,
			allocation:   allocation,
			swarmStatus:  swarmStatus,
			nodeInfo:     node,
			link:         *link,
		}

		initScript, err = initTemplate.Render(context)

		log.Debugln(role, ">>> context.retries =", context.retries, "err=", err, "i=", i)

		if err == nil {
			break
		}

		if context.retries == 0 || i == context.retries {
			log.Warningln("Retries exceeded and error:", err)
			return instanceSpec, err
		}

		log.Infoln("Going to wait for swarm to be ready. i=", i)
		time.Sleep(context.poll)
	}

	log.Debugln(role, "init script:", initScript)

	instanceSpec.Init = initScript

	if instanceSpec.LogicalID != nil {
		if attachments, exists := spec.Attachments[*instanceSpec.LogicalID]; exists {
			instanceSpec.Attachments = append(instanceSpec.Attachments, attachments...)
		}
	}

	// TODO(wfarner): Use the cluster UUID to scope instances for this swarm separately from instances in another
	// swarm.  This will require plumbing back to Scaled (membership tags).
	instanceSpec.Tags["swarm-id"] = swarmID
	link.WriteMap(instanceSpec.Tags)

	return instanceSpec, nil
}

func (s *baseFlavor) Drain(flavorProperties *types.Any, inst instance.Description) error {
	return nil
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
			Name: "SPEC",
			Description: []string{
				"The flavor spec as found in Properties field of the config JSON",
			},
			Func: func() interface{} {
				return c.flavorSpec
			},
		},
		{
			Name: "INSTANCE_LOGICAL_ID",
			Description: []string{
				"The logical id for the instance being prepared.",
				"For cattle (instances with no logical id in allocations), this is empty.",
			},
			Func: func() string {
				if c.instanceSpec.LogicalID != nil {
					return string(*c.instanceSpec.LogicalID)
				}
				return ""
			},
		},
		{
			Name:        "ALLOCATIONS",
			Description: []string{"The allocations contain fields such as the size of the group or the list of logical ids."},
			Func: func() interface{} {
				return c.allocation
			},
		},
		{
			Name:        "INFRAKIT_LABELS",
			Description: []string{"The Docker engine labels to be applied for linking the Docker engine to this instance."},
			Func: func() []string {
				return c.link.KVPairs()
			},
		},
		{
			Name:        "SWARM_MANAGER_IP",
			Description: []string{"IP of the Swarm manager / leader"},
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
			Description: []string{"Returns true if the swarm has been initialized."},
			Func: func() bool {
				if c.nodeInfo == nil {
					return false
				}
				return c.nodeInfo.ManagerStatus != nil
			},
		},
		{
			Name:        "SWARM_JOIN_TOKENS",
			Description: []string{"Returns the swarm JoinTokens object, with either .Manager or .Worker fields"},
			Func: func() (interface{}, error) {
				if c.swarmStatus == nil {
					return nil, fmt.Errorf("cannot prepare: no swarm status")
				}
				return c.swarmStatus.JoinTokens, nil
			},
		},
		{
			Name:        "SWARM_CLUSTER_ID",
			Description: []string{"Returns the swarm cluster UUID"},
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

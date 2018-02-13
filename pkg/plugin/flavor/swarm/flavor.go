package swarm

import (
	"fmt"
	"time"

	docker_types "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/go-connections/tlsconfig"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/docker"
	"golang.org/x/net/context"
)

var (
	log    = logutil.New("module", "flavor/swarm")
	debugV = logutil.V(300)
)

const (
	ebsAttachment string = "ebs"

	// AllInstances as a special logical ID for use in the Attachments map
	AllInstances = instance.LogicalID("*")
)

var defaultTemplateOptions = template.Options{MultiPass: true}

// Spec is the value passed in the `Properties` field of configs
type Spec struct {

	// Attachments indicate the devices that are to be attached to the instance.
	// If the logical ID is '*' (the AllInstances const) then the attachment applies to all instances.
	Attachments map[instance.LogicalID][]instance.Attachment

	// InitScriptTemplateURL overrides the template specified when the plugin started up.
	InitScriptTemplateURL string

	// SwarmJoinIP is the IP for managers and workers to join
	SwarmJoinIP string

	// Labels to apply on the Docker engine
	EngineLabels map[string]string

	// Docker holds the connection params to the Docker engine for join tokens, etc.
	Docker docker.ConnectInfo
}

// DockerClient checks the validity of input spec and connects to Docker engine
func DockerClient(spec Spec) (docker.APIClientCloser, error) {
	if spec.Docker.Host == "" && spec.Docker.TLS == nil {
		return nil, fmt.Errorf("no docker connect info")
	}
	tls := spec.Docker.TLS
	if tls == nil {
		tls = &tlsconfig.Options{}
	}

	return docker.NewClient(spec.Docker.Host, tls)
}

// baseFlavor is the base implementation.  The manager / worker implementations will provide override.
type baseFlavor struct {
	getDockerClient func(Spec) (docker.APIClientCloser, error)
	initScript      *template.Template
	metadataPlugin  metadata.Plugin
	scope           scope.Scope
}

// Runs a poller that periodically samples the swarm status and node info.
func (s *baseFlavor) runMetadataSnapshot(stopSnapshot <-chan struct{}) chan func(map[string]interface{}) {
	// Start a poller to load the snapshot and make that available as metadata
	updateSnapshot := make(chan func(map[string]interface{}))
	go func() {
		tick := time.Tick(1 * time.Second)
		for {
			select {
			case <-tick:
				snapshot := map[string]interface{}{}
				docker, err := s.getDockerClient(Spec{
					Docker: docker.ConnectInfo{
						Host: "unix:///var/run/docker.sock", // defaults to local socket
					},
				})
				if err != nil {
					snapshot["local"] = map[string]interface{}{"error": err}
				} else {
					if status, node, err := swarmState(docker); err != nil {
						snapshot["local"] = map[string]interface{}{"error": err}
					} else {
						snapshot["local"] = map[string]interface{}{
							"status": status,
							"node":   node,
						}
					}
					docker.Close()
				}

				updateSnapshot <- func(view map[string]interface{}) {
					types.Put([]string{"groups"}, snapshot, view)
				}

			case <-stopSnapshot:
				log.Info("Snapshot updater stopped")
				return
			}
		}
	}()
	return updateSnapshot
}

// Keys implements the metadata.Plugin SPI's Keys method
func (s *baseFlavor) Keys(path types.Path) ([]string, error) {
	if s.metadataPlugin != nil {
		return s.metadataPlugin.Keys(path)
	}
	return nil, nil
}

// Get implements the metadata.Plugin SPI's Get method
func (s *baseFlavor) Get(path types.Path) (*types.Any, error) {
	if s.metadataPlugin != nil {
		return s.metadataPlugin.Get(path)
	}
	return nil, nil
}

// Funcs implements the template.FunctionExporter interface that allows the RPC server to expose help on the
// functions it exports
func (s *baseFlavor) Funcs() []template.Function {
	return (&templateContext{}).Funcs()
}

// Validate checks the configuration of flavor plugin.
func (s *baseFlavor) Validate(flavorProperties *types.Any, allocation group.AllocationMethod) error {
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

	return validateIDsAndAttachments(allocation.LogicalIDs, spec.Attachments)
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

	link := types.NewLinkFromMap(inst.Tags)
	if !link.Valid() {
		log.Info("Reporting unhealthy for instance without an association tag", inst.ID)
		return flavor.Unhealthy, nil
	}

	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=%s", link.Label(), link.Value()))

	dockerClient, err := s.getDockerClient(spec)
	if err != nil {
		return flavor.Unknown, err
	}
	defer dockerClient.Close()

	nodes, err := dockerClient.NodeList(context.Background(), docker_types.NodeListOptions{Filters: filter})
	if err != nil {
		return flavor.Unknown, err
	}

	switch {
	case len(nodes) == 0:
		// The instance may not yet be joined, so we consider the health unknown.
		return flavor.Unknown, nil

	case len(nodes) == 1:
		if nodes[0].Status.State != swarm.NodeStateReady {
			log.Warn("Node is not ready",
				"id", nodes[0].ID,
				"hostname", nodes[0].Description.Hostname,
				"state", nodes[0].Status.State)
			return flavor.Unhealthy, nil
		}
		if nodes[0].Spec.Role == swarm.NodeRoleManager {
			if nodes[0].ManagerStatus == nil {
				log.Error("ManagerStatus is not defined",
					"id", nodes[0].ID,
					"hostname", nodes[0].Description.Hostname)
				return flavor.Unhealthy, nil
			}
			if nodes[0].ManagerStatus.Reachability != swarm.ReachabilityReachable {
				log.Warn("Manager node is not reachable",
					"id", nodes[0].ID,
					"hostname", nodes[0].Description.Hostname,
					"reachability", nodes[0].ManagerStatus.Reachability)
				return flavor.Unhealthy, nil
			}
		}
		return flavor.Healthy, nil

	default:
		log.Warn("Found duplicates", "label", link.Value(), "nodes", nodes)
		return flavor.Healthy, nil
	}
}

func (s *baseFlavor) prepare(role string, flavorProperties *types.Any, instanceSpec instance.Spec,
	allocation group.AllocationMethod,
	index group.Index) (instance.Spec, error) {

	spec := Spec{}

	err := flavorProperties.Decode(&spec)
	if err != nil {
		return instanceSpec, err
	}

	initTemplate := s.initScript

	if spec.InitScriptTemplateURL != "" {

		t, err := s.scope.TemplateEngine(spec.InitScriptTemplateURL, defaultTemplateOptions)
		if err != nil {
			return instanceSpec, err
		}

		initTemplate = t
		log.Info("Init template", "url", spec.InitScriptTemplateURL)
	}

	var swarmID, initScript string
	var swarmStatus *swarm.Swarm
	var node *swarm.Node
	var link *types.Link

	for i := 0; ; i++ {
		log.Debug("query docker swarm", "role", role, "index", i)

		dockerClient, err := s.getDockerClient(spec)
		if err != nil {
			log.Error("Cannot connect to Docker", "err", err)
			continue
		}

		swarmStatus, node, err = swarmState(dockerClient)
		if err != nil {
			log.Error("Cannot prepare", "err", err)
		}

		if dockerClient != nil {
			dockerClient.Close()
		}

		swarmID = "?"
		if swarmStatus != nil {
			swarmID = swarmStatus.ID
		}

		link = types.NewLink().WithContext("swarm::" + swarmID + "::" + role)
		context := &templateContext{
			flavorSpec:   spec,
			instanceSpec: instanceSpec,
			allocation:   allocation,
			index:        index,
			swarmStatus:  swarmStatus,
			nodeInfo:     node,
			link:         *link,
		}

		initScript, err = initTemplate.Render(context)
		log.Debug("retries", "role", role, "retries", context.retries, "err", err, "i", i)

		if err == nil {
			break
		}

		if context.retries == 0 || i == context.retries {
			log.Warn("Retries exceeded", "err", err)
			return instanceSpec, err
		}

		log.Info("Going to wait for swarm to be ready", "i", i)
		time.Sleep(context.poll)
	}

	log.Debug("init script", "role", role, "script", initScript, "V", debugV)

	instanceSpec.Init = initScript

	if instanceSpec.LogicalID != nil {
		if attachments, exists := spec.Attachments[*instanceSpec.LogicalID]; exists {
			instanceSpec.Attachments = append(instanceSpec.Attachments, attachments...)
		}
	}

	// look for the AllInstances logicalID in shared attachments
	for logicalID, attachments := range spec.Attachments {
		if logicalID == AllInstances {
			instanceSpec.Attachments = append(instanceSpec.Attachments, attachments...)
		}
	}

	if instanceSpec.Tags == nil {
		instanceSpec.Tags = map[string]string{}
	}

	// TODO(wfarner): Use the cluster UUID to scope instances for this swarm separately from instances in another
	// swarm.  This will require plumbing back to Scaled (membership tags).
	instanceSpec.Tags[flavor.ClusterIDTag] = swarmID
	link.WriteMap(instanceSpec.Tags)

	return instanceSpec, nil
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
		if _, exists := idsMap[id]; !exists && id != AllInstances {
			return fmt.Errorf("LogicalID %v used for an attachment but is not in group LogicalIDs", id)
		}
	}

	// Only EBS attachments are supported.
	for _, atts := range attachments {
		for _, attachment := range atts {
			if attachment.Type == "" {
				return fmt.Errorf("no attachment type")
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

func swarmState(docker docker.APIClientCloser) (status *swarm.Swarm, node *swarm.Node, err error) {
	ctx := context.Background()
	info, err := docker.Info(ctx)
	if err != nil {
		log.Error("Err docker info", "err", err)
		status = nil
		node = nil
		return
	}
	n, _, err := docker.NodeInspectWithRaw(ctx, info.Swarm.NodeID)
	if err != nil {
		log.Error("Err node inspect", "err", err)
		return
	}

	node = &n

	s, err := docker.SwarmInspect(ctx)
	if err != nil {
		log.Error("Err swarm inspect", "err", err)
		return
	}
	status = &s
	return
}

type templateContext struct {
	flavorSpec   Spec
	instanceSpec instance.Spec
	allocation   group.AllocationMethod
	index        group.Index
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
			Name:        "INDEX",
			Description: []string{"The launch index of this instance. Contains the group ID and the sequence number of instance."},
			Func: func() interface{} {
				return c.index
			},
		},
		{
			Name:        "INFRAKIT_LABELS",
			Description: []string{"The Docker engine labels to be applied for linking the Docker engine to this instance, as well as those defined in the flavor spec."},
			Func: func() []string {
				if len(c.flavorSpec.EngineLabels) > 0 {
					out := []string{}
					for k, v := range c.flavorSpec.EngineLabels {
						out = append(out, fmt.Sprintf("%s=%s", k, v))
					}
					for k, v := range c.link.Map() {
						out = append(out, fmt.Sprintf("%s=%s", k, v))
					}
					return out
				}
				return c.link.KVPairs()
			},
		},
		{
			Name:        "SWARM_MANAGER_ADDR",
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

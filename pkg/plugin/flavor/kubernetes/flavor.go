package kubernetes

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubediscovery "k8s.io/kubernetes/cmd/kubeadm/app/discovery"
	kubetoken "k8s.io/kubernetes/cmd/kubeadm/app/util/token"
)

var (
	log = logutil.New("module", "flavor/kubernetes")

	// DefaultTemplateOptions contains the default values to use for templates
	DefaultTemplateOptions = template.Options{MultiPass: true}
)

const (
	// AllInstances as a special logical ID for use in the Attachments map
	AllInstances = instance.LogicalID("*")
)

// Options capture static plugin-related settings
type Options struct {

	// ConfigDir is the location where the plugin uses to store some state like join token
	ConfigDir string

	// DefaultManagerInitScriptTemplate is the URL for the default control plane template url.
	// It's overridden by the init script template url specified in the properties.
	DefaultManagerInitScriptTemplate types.URL

	// DefaultWorkerInitScriptTemplate is the URL for the default data plane (workers) template url.
	// This is overridden by the template specified in the properties
	DefaultWorkerInitScriptTemplate types.URL

	// MultiMaster specifies if the control plane supports multi master
	MultiMaster bool
}

// Spec is the value passed in the `Properties` field of configs
type Spec struct {

	// Attachments indicate the devices that are to be attached to the instance.
	// If the logical ID is '*' (the AllInstances const) then the attachment applies to all instances.
	Attachments map[instance.LogicalID][]instance.Attachment

	// InitScriptTemplateURL overrides the template specified when the plugin started up.
	InitScriptTemplateURL string

	// KubeJoinIP is the IP for managers and workers to join
	KubeJoinIP string

	// KubeBindPort is the IP for managers and workers to join
	KubeBindPort int

	// KubeAddOns is the networking and network policy provider
	KubeAddOns []AddOnInfo

	// KubeClusterID is the ID of Kubernetes Cluster you will deploy.
	KubeClusterID string

	// SkipManagerValidation is skip to check manager for worker
	SkipManagerValidation bool

	// ControlPlane are the nodes that make up the Kube control plane.  This is a list of logical IDs
	// that correspond to the manager group of nodes.  For a single master setup, this slice should
	// contain a single element of the IP address or some identifier for the master node.
	ControlPlane []instance.LogicalID
}

// AddOnInfo is info mation of kubernetes add on information. Type is add on type network and visualise. See https://kubernetes.io/docs/concepts/cluster-administration/addons/
type AddOnInfo struct {
	Name string
	Type string
	Path string
}

type baseFlavor struct {
	initScript     *template.Template
	metadataPlugin metadata.Plugin
	scope          scope.Scope
	options        Options
}

func getTemplate(url types.URL, defaultTemplate string, opts template.Options) (t *template.Template, err error) {
	if url.String() == "" {
		t, err = template.NewTemplate("str://"+defaultTemplate, opts)
		return
	}
	t, err = template.NewTemplate(url.String(), opts)
	return
}

func checkKubeAPIServer(cfg kubeadmapi.NodeConfiguration, check chan error, clcfg *clientcmdapi.Config) {
	defer close(check)
	clcfg, err := kubediscovery.For(&cfg)
	if err != nil {
		log.Warn("Cannot connect to Kubernetes API server", "err", err)
		check <- err
	}
	check <- nil
	return
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

	if spec.KubeJoinIP == "" {
		return fmt.Errorf("no kube join info")
	}

	if spec.KubeClusterID == "" {
		return fmt.Errorf("no Kube Cluster ID")
	}

	if spec.InitScriptTemplateURL != "" {
		_, err := template.NewTemplate(spec.InitScriptTemplateURL, DefaultTemplateOptions)
		if err != nil {
			return err
		}
	}

	// If we support only single master then make sure the spec conforms to it
	if s.options.MultiMaster {
		if len(allocation.LogicalIDs) == 0 {
			return fmt.Errorf("must have at least one logical id for control plane")
		}
		if len(allocation.LogicalIDs)%2 == 0 {
			return fmt.Errorf("must have odd number of logical ids: %v", allocation.LogicalIDs)
		}

		if len(spec.ControlPlane) == 0 {
			return fmt.Errorf("must specify ControlPlane in spec: %v", spec)
		}
	}

	if len(spec.ControlPlane) > 0 && len(spec.ControlPlane)%2 == 0 {
		return fmt.Errorf("must have odd number of control plane ids: %v", spec.ControlPlane)
	}

	// We require the control plane logical Ids to be a strict subset of the allocation's instance IDs, if specified.
	if len(allocation.LogicalIDs) > 0 && len(spec.ControlPlane) > 0 {
		if !strictSubset(spec.ControlPlane, allocation.LogicalIDs) {
			return fmt.Errorf("ControlPlane ids must be equal or strict subset of allocations logical IDs: %v vs %v",
				spec.ControlPlane, allocation.LogicalIDs)
		}
	}

	return validateIDsAndAttachments(allocation.LogicalIDs, spec.Attachments)
}

func strictSubset(a, b []instance.LogicalID) bool {
	bm := map[instance.LogicalID]struct{}{}

	for _, bb := range b {
		bm[bb] = struct{}{}
	}

	for _, aa := range a {
		_, has := bm[aa]
		if !has {
			return false
		}
	}
	return true
}

func isControlPlane(id *instance.LogicalID, cplane []instance.LogicalID) (int, bool) {
	if id == nil {
		return -1, false
	}
	for i, node := range cplane {
		if *id == node {
			return i, true
		}
	}
	return -1, false
}

// Healthy determines whether an instance is healthy.
func (s *baseFlavor) Healthy(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
	return flavor.Healthy, nil
}

// Keys implements the metadata.Plugin SPI's Keys method
func (s *baseFlavor) Keys(path types.Path) ([]string, error) {
	return nil, nil
}

// Get implements the metadata.Plugin SPI's Get method
func (s *baseFlavor) Get(path types.Path) (*types.Any, error) {
	if s.metadataPlugin != nil {
		return s.metadataPlugin.Get(path)
	}
	return nil, nil
}

func (s *baseFlavor) prepare(role string, flavorProperties *types.Any, instanceSpec instance.Spec,
	allocation group.AllocationMethod,
	index group.Index) (instance.Spec, error) {
	spec := Spec{}

	log.Debug("prepare", "role", role, "properties", flavorProperties, "spec", instanceSpec, "alloc", allocation,
		"index", index)

	err := flavorProperties.Decode(&spec)
	if err != nil {
		return instanceSpec, err
	}

	clDir := path.Join(s.options.ConfigDir, "infrakit-kube-"+spec.KubeClusterID)
	tokenPath := path.Join(clDir, "kubeadm-token")

	var token string
	var bootstrap bool
	var worker bool
	switch role {
	case "manager":
		index, is := isControlPlane(instanceSpec.LogicalID, spec.ControlPlane)
		if !is {
			worker = true
			break
		}

		if index == 0 {
			bootstrap = true
		}

		if _, err := os.Stat(clDir); err != nil {
			if err := os.MkdirAll(clDir, 0777); err != nil {
				log.Error("can't make dir", "dir", clDir, "err", err)
				return instanceSpec, err
			}
		}

		if _, err := os.Stat(tokenPath); err == nil {
			var btoken []byte
			if btoken, err = ioutil.ReadFile(tokenPath); err != nil {
				log.Error("can't read token", "path", tokenPath, "err", err)
				return instanceSpec, err
			}
			token = string(btoken)
		} else {

			log.Info("generating token", "path", tokenPath)

			token, err = kubetoken.GenerateToken()
			if err != nil {
				return instanceSpec, err
			}
			if err := ioutil.WriteFile(tokenPath, []byte(token), 0666); err != nil {
				log.Error("can't write token", "path", tokenPath, "err", err)
				return instanceSpec, err
			}
			log.Info("written token", "path", tokenPath)

		}

		token = strings.TrimRight(token, "\n")

	case "worker":
		worker = true
	}

	if worker {

		d, err := ioutil.ReadFile(tokenPath)
		if err != nil {
			log.Error("can't find token at path", "path", tokenPath, "err", err)
			return instanceSpec, err
		}
		token = string(d)
		token = strings.TrimRight(token, "\n")

		if !spec.SkipManagerValidation {
			cfg := kubeadmapi.NodeConfiguration{
				DiscoveryTokenAPIServers: []string{spec.KubeJoinIP + ":" + strconv.Itoa(spec.KubeBindPort)},
				DiscoveryToken:           token,
			}
			c := make(chan error)
			var clcfg *clientcmdapi.Config
			go checkKubeAPIServer(cfg, c, clcfg)
			select {
			case apicheck := <-c:
				if apicheck != nil {
					return instanceSpec, err
				}
			case <-time.After(120 * time.Second):
				log.Warn("Connection time out for Kubernetes API server")
				log.Warn("If Kubernetes API server is not reachable, you can set `SkipManagerValidation: true` in your configuration.")
				return instanceSpec, fmt.Errorf("Connection time out for Kubernetes API server %s", spec.KubeJoinIP+":"+strconv.Itoa(spec.KubeBindPort))
			}
		}
	}

	initTemplate := s.initScript
	var initScript string
	var link *types.Link

	if spec.InitScriptTemplateURL != "" {

		t, err := s.scope.TemplateEngine(spec.InitScriptTemplateURL, DefaultTemplateOptions)
		if err != nil {
			log.Error("error processing template", "template", spec.InitScriptTemplateURL, "err", err)
			return instanceSpec, err
		}

		initTemplate = t
		log.Info("Init script template", "template", spec.InitScriptTemplateURL)
	}

	link = types.NewLink().WithContext("kubernetes::" + role)
	context := &templateContext{
		flavorSpec:   spec,
		instanceSpec: instanceSpec,
		allocation:   allocation,
		index:        index,
		link:         *link,
		joinToken:    token,
		bootstrap:    bootstrap,
		worker:       worker,
	}

	initScript, err = initTemplate.Render(context)
	instanceSpec.Init = initScript
	log.Debug("Init script", "content", initScript)
	return instanceSpec, nil
}

// TODO - call kubectl drain and then delete node
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

type templateContext struct {
	flavorSpec   Spec
	instanceSpec instance.Spec
	allocation   group.AllocationMethod
	index        group.Index
	link         types.Link
	retries      int
	poll         time.Duration
	joinToken    string
	worker       bool
	bootstrap    bool
}

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
			Name:        "BOOTSTRAP",
			Description: []string{"True if this is a bootstrapper node"},
			Func: func() interface{} {
				return c.bootstrap
			},
		},
		{
			Name:        "WORKER",
			Description: []string{"True if this is a member of the data plane (workers)"},
			Func: func() interface{} {
				return c.worker
			},
		},
		{
			Name:        "KUBEADM_JOIN_TOKEN",
			Description: []string{"Returns the kubeadm JoinToken object"},
			Func: func() (interface{}, error) {
				return c.joinToken, nil
			},
		},
		{
			Name:        "KUBE_JOIN_IP",
			Description: []string{"Returns the kube advertise IP"},
			Func: func() (interface{}, error) {
				return c.flavorSpec.KubeJoinIP, nil
			},
		},
		{
			Name:        "BIND_PORT",
			Description: []string{"Returns the kubeadm JoinToken object"},
			Func: func() (interface{}, error) {
				return c.flavorSpec.KubeBindPort, nil
			},
		},
		{
			Name:        "ADDON",
			Description: []string{"Returns the kubernetes addon"},
			Func: func(addonType string) (interface{}, error) {
				aPath := ""
				for _, a := range c.flavorSpec.KubeAddOns {
					if a.Type == addonType {
						aPath = a.Path
					}
				}

				return aPath, nil
			},
		},
	}
}

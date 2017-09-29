package kubernetes

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	metadata_template "github.com/docker/infrakit/pkg/plugin/metadata/template"
	"github.com/docker/infrakit/pkg/spi/flavor"
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
	plugins        func() discovery.Plugins
	kubeConfDir    string
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
func (s *baseFlavor) Validate(flavorProperties *types.Any, allocation group_types.AllocationMethod) error {
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

	return validateIDsAndAttachments(allocation.LogicalIDs, spec.Attachments)
}

// Healthy determines whether an instance is healthy.
func (s *baseFlavor) Healthy(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
	return flavor.Healthy, nil
}

// List implements the metadata.Plugin SPI's List method
func (s *baseFlavor) List(path types.Path) ([]string, error) {
	return nil, nil
}

// Get implements the metadata.Plugin SPI's List method
func (s *baseFlavor) Get(path types.Path) (*types.Any, error) {
	if s.metadataPlugin != nil {
		return s.metadataPlugin.Get(path)
	}
	return nil, nil
}

func (s *baseFlavor) prepare(role string, flavorProperties *types.Any, instanceSpec instance.Spec,
	allocation group_types.AllocationMethod,
	index group_types.Index) (instance.Spec, error) {
	spec := Spec{}

	log.Debug("prepare", "role", role, "properties", flavorProperties, "spec", instanceSpec, "alloc", allocation,
		"index", index)

	if s.plugins == nil {
		return instanceSpec, fmt.Errorf("no plugin discovery")
	}

	err := flavorProperties.Decode(&spec)
	if err != nil {
		return instanceSpec, err
	}
	clDir := path.Join(s.kubeConfDir, "infrakit-kube-"+spec.KubeClusterID)

	var token string
	switch role {
	case "manager":
		if _, err := os.Stat(clDir); err != nil {
			if err := os.Mkdir(clDir, 0777); err != nil {
				return instanceSpec, err
			}
		}
		f := path.Join(clDir, "kubeadm-token")
		if _, err := os.Stat(f); err == nil {
			var btoken []byte
			if btoken, err = ioutil.ReadFile(f); err != nil {
				return instanceSpec, err
			}
			token = string(btoken)
		} else {
			token, err = kubetoken.GenerateToken()
			if err != nil {
				return instanceSpec, err
			}
			if err := ioutil.WriteFile(f, []byte(token), 0666); err != nil {
				return instanceSpec, err
			}
		}
		token = strings.TrimRight(token, "\n")
	case "worker":
		f := path.Join(clDir, "kubeadm-token")
		d, err := ioutil.ReadFile(f)
		if err != nil {
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

		t, err := template.NewTemplate(spec.InitScriptTemplateURL, DefaultTemplateOptions)
		if err != nil {
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
	}

	initTemplate.WithFunctions(func() []template.Function {
		return []template.Function{
			{
				Name: "metadata",
				Description: []string{
					"Metadata function takes a path of the form \"plugin_name/path/to/data\"",
					"and calls GET on the plugin with the path \"path/to/data\".",
					"It's identical to the CLI command infrakit metadata cat ...",
				},
				Func: metadata_template.MetadataFunc(s.plugins),
			},
		}
	})
	initScript, err = initTemplate.Render(context)
	instanceSpec.Init = initScript
	log.Debug("Init script", "content", initScript)
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
	allocation   group_types.AllocationMethod
	index        group_types.Index
	link         types.Link
	retries      int
	poll         time.Duration
	joinToken    string
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

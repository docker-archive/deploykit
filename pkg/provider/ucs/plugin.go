package ucs

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/CiscoUcs/UCS-Terraform/ucsclient"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "cli/x")

// Options capture the config parameters required to create the plugin
type Options struct {
	UCSUrl    string
	UCSUser   string
	UCSPass   string
	UCSCookie string
}

//miniFSM for managing the provisioning -> provisioned state
type provisioningFSM struct {
	countdown    int64             // ideally will be a counter of minutes / seconds
	tags         map[string]string // tags that will be passed back per a describe function
	instanceName string            // name that we will use as a lookup to the actual backend that is privisioning
}

// Spec is just whatever that can be unmarshalled into a generic JSON map
type Spec map[string]interface{}

// This contains the the details for the oneview instance
type plugin struct {
	fsm    []provisioningFSM
	client *ucsclient.UCSClient
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// NewUCSInstancePlugin will take the cmdline/env configuration
func NewUCSInstancePlugin(ucsOptions Options) instance.Plugin {

	ucsConfig := ucsclient.Config{
		AppName:               "UCS",
		IpAddress:             ucsOptions.UCSUrl,
		Username:              ucsOptions.UCSUser,
		Password:              ucsOptions.UCSPass,
		TslInsecureSkipVerify: true, // TODO - Make configurable
		LogFilename:           "./ucs.log",
		LogLevel:              0,
	}

	client := ucsConfig.Client()
	err := client.Login()
	// Exit with an error if we can't connect to Cisco UCS Domain
	if err != nil {
		log.Crit("Error Logging into Cisco UCS Domain")
		os.Exit(-1)
	}

	log.Info("Succesfully logged in to UCS Domain")

	return &plugin{
		client: client,
	}
}

// Info returns a vendor specific name and version
func (p *plugin) VendorInfo() *spi.VendorInfo {
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-instance-ucs",
			Version: "0.6.0",
		},
		URL: "https://github.com/docker/infrakit",
	}
}

// ExampleProperties returns the properties / config of this plugin
func (p *plugin) ExampleProperties() *types.Any {
	any, err := types.AnyValue(Spec{
		"exampleString": "a_string",
		"exampleBool":   true,
		"exampleInt":    1,
	})
	if err != nil {
		return nil
	}
	return any
}

// Validate performs local validation on a provision request.
func (p *plugin) Validate(req *types.Any) error {
	log.Debug("validate", req.String())

	spec := Spec{}
	if err := req.Decode(&spec); err != nil {
		return err
	}

	log.Debug("Validated:", spec)
	return nil
}

// Provision creates a new instance based on the spec.
func (p *plugin) Provision(spec instance.Spec) (*instance.ID, error) {

	var properties map[string]interface{}

	if spec.Properties != nil {
		if err := spec.Properties.Decode(&properties); err != nil {
			return nil, fmt.Errorf("Invalid instance properties: %s", err)
		}
	}

	instanceName := instance.ID(fmt.Sprintf("InfraKit-%d", rand.Int63()))

	return &instanceName, nil
}

// Label labels the instance
func (p *plugin) Label(instance instance.ID, labels map[string]string) error {
	return fmt.Errorf("Cisco UCS label updates are not implemented yet")
}

// Destroy terminates an existing instance.
func (p *plugin) Destroy(instance instance.ID, context instance.Context) error {
	log.Info("Currently running %s on instance: %v", context, instance)
	return nil
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
// TODO - need to define the fitlering of tags => AND or OR of matches?
func (p *plugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	log.Debug("describe-instances", tags)
	results := []instance.Description{}

	return results, nil
}

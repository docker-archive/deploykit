package instance

import (
	"encoding/json"
	"errors"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/codedellemc/gorackhd/client/nodes"
	"github.com/codedellemc/gorackhd/client/skus"
	"github.com/codedellemc/infrakit.rackhd/monorail"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/go-openapi/runtime"
)

type rackHDInstancePlugin struct {
	Client   monorail.Iface
	Username string
	Password string
}

type RackHDWorkflow struct {
	Name    string
	Options interface{}
}

// RackHDProperties are the details of the RackHD provision request to be processed by RackHD
type RackHDProperties struct {
	Workflow RackHDWorkflow
	SKUName  string
}

// NewInstancePlugin creates a new plugin that creates instances in RackHD.
func NewInstancePlugin(client monorail.Iface, username string, password string) instance.Plugin {
	return &rackHDInstancePlugin{Client: client, Username: username, Password: password}
}

func (p rackHDInstancePlugin) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
	return nil, nil
}

func (p rackHDInstancePlugin) Destroy(id instance.ID) error {
	return nil
}

func (p rackHDInstancePlugin) Label(id instance.ID, labels map[string]string) error {
	return nil
}

func (p rackHDInstancePlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	var instanceID instance.ID
	if spec.Properties == nil {
		return &instanceID, errors.New("Properties must be set")
	}
	props := RackHDProperties{}
	err := json.Unmarshal(*spec.Properties, &props)
	if err != nil {
		return &instanceID, fmt.Errorf("Invalid input formatting: %s", err)
	}

	skuName := props.SKUName
	auth, nil := p.Client.Login(p.Username, p.Password)
	/*
		if err != nil {
			return &instanceID, fmt.Errorf("Unable to log into RackHD as %s: %s", p.Username, err)
		}
	*/
	log.Infof("Logged into RackHD service as %s", p.Username)

	skuID, err := p.getSKUIDForName(skuName, auth)
	if err != nil {
		return &instanceID, fmt.Errorf("Unable to lookup SKU ID: %s", err)
	}
	log.Infof("Found SKU ID, %s, for name \"%s\"", skuID, skuName)

	nodeID, err := p.getAvailableNodeIDForSKU(skuID, auth)
	if err != nil {
		return &instanceID, fmt.Errorf("Unable to select node ID for SKU ID, %s. %s", skuID, err)
	}
	log.Infof("Found available node ID: %s", nodeID)

	err = p.applyWorkflowToNode(props.Workflow, nodeID, auth)
	if err != nil {
		return &instanceID, fmt.Errorf("Unable to apply workflow: %s", err)
	}

	instanceID = instance.ID(nodeID)
	return &instanceID, nil
}

func (p rackHDInstancePlugin) Validate(req *types.Any) error {
	return nil
}

func (p rackHDInstancePlugin) getSKUIDForName(skuName string, auth runtime.ClientAuthInfoWriter) (string, error) {
	if skuName == "" {
		return "", fmt.Errorf("no SKU name provided")
	}

	skuList, nil := p.Client.Skus().GetSkus(skus.NewGetSkusParams(), auth)
	var skuID string
	for _, sku1 := range skuList.Payload {
		if string(sku1.Name) == skuName {
			skuID = string(sku1.ID)
			break
		}
	}
	if skuID == "" {
		return "", fmt.Errorf("required SKU not found. Wanted %s, but not found",
			skuName)
	}
	return skuID, nil
}

func (p rackHDInstancePlugin) getAvailableNodeIDForSKU(skuID string, auth runtime.ClientAuthInfoWriter) (string, error) {
	getNodesParams := skus.NewGetSkusIdentifierNodesParams().WithIdentifier(skuID)
	nodes, nil := p.Client.Skus().GetSkusIdentifierNodes(getNodesParams, auth)
	var nodeID string
	for _, node1 := range nodes.Payload {
		// Consumed nodes must be un-tagged to be provisioned
		if string(node1.Type) == "compute" && len(node1.Tags) == 0 {
			nodeID = string(node1.ID)
			break
		}
	}
	if nodeID == "" {
		return "", fmt.Errorf("no eligible nodes found matching SKU ID: %s", skuID)
	}
	return nodeID, nil
}

func (p rackHDInstancePlugin) applyWorkflowToNode(workflow RackHDWorkflow, nodeID string, auth runtime.ClientAuthInfoWriter) error {
	log.Infof("%s", workflow)
	if workflow.Name == "" {
		return fmt.Errorf("No workflow name provided")
	}

	body := make(map[string]interface{})
	body["name"] = workflow.Name
	body["options"] = workflow.Options

	params := nodes.NewPostNodesIdentifierWorkflowsParams().
		WithIdentifier(nodeID).
		WithName(workflow.Name).
		WithBody(body)
	log.Infof("POST PARAMS: %s", params)
	_, err := p.Client.Nodes().PostNodesIdentifierWorkflows(params, auth)
	if err != nil {
		return err
	}
	return nil
}

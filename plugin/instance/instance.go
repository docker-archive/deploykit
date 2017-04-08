package instance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/codedellemc/gorackhd/client/nodes"
	"github.com/codedellemc/gorackhd/client/skus"
	"github.com/codedellemc/gorackhd/client/tags"
	"github.com/codedellemc/gorackhd/models"
	"github.com/codedellemc/infrakit.rackhd/monorail"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/go-openapi/runtime"
)

// rackHDInstancePlugin is the public plugin client struct
type rackHDInstancePlugin struct {
	Client   monorail.Iface
	Username string
	Password string
}

// RackHDWorkflow are the details of the Workflow to be applied
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
	return rackHDInstancePlugin{Client: client, Username: username, Password: password}
}

// DescribeInstances Lists the instances running in RackHD by tags
func (p rackHDInstancePlugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	auth, err := p.Client.Login(p.Username, p.Password)
	if err != nil {
		return nil, fmt.Errorf("Unable to log into RackHD as %s: %s", p.Username, err)
	}
	log.Infof("Logged into RackHD service as %s", p.Username)

	nodes, nil := p.Client.Nodes().GetNodes(nodes.NewGetNodesParams(), auth)
	log.Infof("NODES L56: %s", nodes)

	descriptions := []instance.Description{}
	for _, node := range nodes.Payload {
		nodeTags, err := getTagMapForNode(node)
		if err != nil {
			return descriptions, err
		}
		keep := true
		for tagKey, tagVal := range tags {
			if nodeTags[tagKey] != tagVal {
				keep = false
			}
		}
		logID := instance.LogicalID(node.ID)
		if keep {
			descriptions = append(descriptions, instance.Description{
				ID:        instance.ID(node.ID),
				LogicalID: &logID,
				Tags:      nodeTags,
			})
		}
	}
	return descriptions, nil
}

func getTagMapForNode(node *models.Node) (map[string]string, error) {
	tags := make(map[string]string)
	for _, tag := range node.Tags {
		if t, ok := tag.(string); ok {
			tagSlice := strings.SplitN(t, "=", 2)
			// Only worry about tags with a key/value format:w
			if len(tagSlice) == 2 {
				tags[tagSlice[0]] = tagSlice[1]
			}
		} else {
			return nil, fmt.Errorf("Cannot convert tag to string: %s", tag)
		}
	}
	return tags, nil
}

// Destroy reformats a RackHD instance and performs a secure erase of the system
func (p rackHDInstancePlugin) Destroy(id instance.ID) error {
	auth, err := p.Client.Login(p.Username, p.Password)
	if err != nil {
		return fmt.Errorf("Unable to log into RackHD as %s: %s", p.Username, err)
	}
	log.Infof("Logged into RackHD service as %s", p.Username)

	options := make(map[string]interface{})
	options["useSecureErase"] = true
	workflow := RackHDWorkflow{Name: "Graph.Bootstrap.Decommission.Node", Options: options}

	err = p.applyWorkflowToNode(workflow, string(id), auth)
	if err != nil {
		return fmt.Errorf("Unable to apply decommision workflow: %s", err)
	}

	return nil
}

// Label writes tags with the infrakit metadata to the RackHD instance
func (p rackHDInstancePlugin) Label(id instance.ID, labels map[string]string) error {
	auth, err := p.Client.Login(p.Username, p.Password)
	if err != nil {
		return fmt.Errorf("Unable to log into RackHD as %s: %s", p.Username, err)
	}
	log.Infof("Logged into RackHD service as %s", p.Username)

	var tagList []string
	for k, v := range labels {
		var tag bytes.Buffer
		tag.WriteString(k)
		tag.WriteString("=")
		tag.WriteString(v)
		tagList = append(tagList, tag.String())
	}
	tagParams := tags.NewPatchNodesIdentifierTagsParams().
		WithIdentifier(string(id)).
		WithBody(tagList)

	_, err = p.Client.Tags().PatchNodesIdentifierTags(tagParams, auth)
	if err != nil {
		return err
	}
	return nil
}

// Provision posts a new workflow to an existing RackHD instance
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
	auth, err := p.Client.Login(p.Username, p.Password)
	if err != nil {
		return &instanceID, fmt.Errorf("Unable to log into RackHD as %s: %s", p.Username, err)
	}
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

	tagParams := tags.NewPatchNodesIdentifierTagsParams().
		WithIdentifier(nodeID).
		WithBody([]string{"infrakitLocked"})

	_, err = p.Client.Tags().PatchNodesIdentifierTags(tagParams, auth)
	if err != nil {
		return nil, err
	}

	err = p.applyWorkflowToNode(props.Workflow, nodeID, auth)
	if err != nil {
		return &instanceID, fmt.Errorf("Unable to apply workflow: %s", err)
	}

	instanceID = instance.ID(nodeID)
	return &instanceID, nil
}

// Validate ensures that the specified instances are running within RackHD
func (p rackHDInstancePlugin) Validate(req *types.Any) error {
	parsed := RackHDProperties{}
	req.Decode(&parsed)

	if parsed.Workflow.Name == "" {
		return fmt.Errorf("no-workflow:%s", req.String())
	}

	if parsed.SKUName == "" {
		return fmt.Errorf("no-sku-name:%s", req.String())
	}
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

package instance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/go-openapi/runtime"
	"github.com/spiegela/gorackhd/client/nodes"
	"github.com/spiegela/gorackhd/client/skus"
	"github.com/spiegela/gorackhd/models"
	"github.com/spiegela/gorackhd/monorail"
)

// rackHDInstancePlugin is the public plugin client struct
type rackHDInstancePlugin struct {
	Client   monorail.Iface
	Username string
	Password string
}

// RackHDProperties are the details of the RackHD provision request to be processed by RackHD
type RackHDProperties struct {
	Workflow *models.PostNodeWorkflow
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

	nodes, err := p.Client.Nodes().NodesGetAll(nodes.NewNodesGetAllParams(), auth)
	if err != nil {
		return nil, err
	}

	descriptions := []instance.Description{}
	for _, node := range nodes.Payload {
		if node.Type == "compute" {
			nodeTags, err := p.getTagMapForNode(node, auth)
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
	}
	log.Infof("Instance descriptions retrieved as %s", descriptions)
	return descriptions, nil
}

func (p rackHDInstancePlugin) getTagMapForNode(node *models.Node20Node, auth runtime.ClientAuthInfoWriter) (map[string]string, error) {
	getTagsParams := nodes.NewNodesGetTagsByIDParams().
		WithIdentifier(node.ID)
	getTagsResp, err := p.Client.Nodes().NodesGetTagsByID(getTagsParams, auth)
	if err != nil {
		return nil, err
	}

	tags := make(map[string]string)
	for _, tag := range getTagsResp.Payload {
		tagSlice := strings.SplitN(tag, "=", 2)
		// Only worry about tags with a key/value format:w
		if len(tagSlice) == 2 {
			tags[tagSlice[0]] = tagSlice[1]
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
	workflow := models.PostNodeWorkflow{Name: "Graph.Bootstrap.Decommission.Node", Options: &models.PostNodeWorkflowOptions{Defaults: options}}
	log.Infof("Posted destruction workflow to instance %s", id)

	err = p.applyWorkflowToNode(&workflow, string(id), auth)
	if err != nil {
		return fmt.Errorf("Unable to apply decommision workflow: %s", err)
	}
	log.Infof("Removing node %s from RackHD", id)
	nodeDelParams := nodes.NewNodesDelByIDParams().
		WithIdentifier(string(id))
	p.Client.Nodes().NodesDelByID(nodeDelParams, auth)

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
	log.Infof("Writing tags to node %s: %s", id, tagList)
	tagParams := nodes.NewNodesPatchTagByIDParams().
		WithIdentifier(string(id)).
		WithTags(&models.NodesPatchTags{Tags: tagList})

	_, err = p.Client.Nodes().NodesPatchTagByID(tagParams, auth)
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
		log.Infof("Unable to select node ID for SKU ID, %s. %s", skuID, err)
		return &instanceID, fmt.Errorf("Unable to select node ID for SKU ID, %s. %s", skuID, err)
	}
	log.Infof("Found available node ID: %s", nodeID)

	log.Infof("Writing tags to node ID %s: %s", nodeID, spec.Tags)
	p.Label(instance.ID(nodeID), spec.Tags)

	err = p.applyWorkflowToNode(props.Workflow, nodeID, auth)
	if err != nil {
		return &instanceID, fmt.Errorf("Unable to apply workflow: %s", err)
	}

	instanceID = instance.ID(nodeID)

	log.Infof("Applying init to node ID %s: %s", nodeID, spec.Init)
	return &instanceID, nil
}

// Validate ensures that the specified instances are running within RackHD
func (p rackHDInstancePlugin) Validate(req *types.Any) error {
	parsed := RackHDProperties{}
	req.Decode(&parsed)

	if parsed.Workflow == nil || parsed.Workflow.Name == "" {
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

	skuList, nil := p.Client.Skus().SkusGet(skus.NewSkusGetParams(), auth)
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
	getNodesParams := skus.NewSkusIDGetNodesParams().WithIdentifier(skuID)
	nodes, err := p.Client.Skus().SkusIDGetNodes(getNodesParams, auth)
	if err != nil {
		return "", err
	}
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

func (p rackHDInstancePlugin) applyWorkflowToNode(workflow *models.PostNodeWorkflow, nodeID string, auth runtime.ClientAuthInfoWriter) error {
	if workflow.Name == "" {
		return fmt.Errorf("No workflow name provided")
	}

	params := nodes.NewNodesPostWorkflowByIDParams().
		WithIdentifier(nodeID).
		WithName(&workflow.Name).
		WithBody(workflow)
	_, err := p.Client.Nodes().NodesPostWorkflowByID(params, auth)
	if err != nil {
		return err
	}
	return nil
}

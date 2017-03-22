package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	maas "github.com/juju/gomaasapi"
	"net/url"
)

// NewMaasPlugin creates an instance plugin for MaaS.
func NewMaasPlugin(dir string, key string, url string, version string) instance.Plugin {
	var err error
	var authClient *maas.Client
	if key != "" {
		authClient, err = maas.NewAuthenticatedClient(url, key, version)
	} else {
		authClient, err = maas.NewAnonymousClient(url, version)
	}
	if err != nil {
		return nil
	}
	maasobj := maas.NewMAAS(*authClient)
	return &maasPlugin{MaasfilesDir: dir, MaasObj: maasobj}
}

type maasPlugin struct {
	MaasfilesDir string
	MaasObj      *maas.MAASObject
}

// Validate performs local validation on a provision request.
func (m maasPlugin) Validate(req *types.Any) error {
	return nil
}

func (m maasPlugin) convertSpecToMaasParam(spec map[string]interface{}) url.Values {
	param := url.Values{}
	return param
}

func (m maasPlugin) addTagsToNode(systemID string, tags map[string]string) error {
	tagListing := m.MaasObj.GetSubObject("tags")
	for tag, value := range tags {
		tagObj, err := tagListing.GetSubObject(tag).Get()
		if err != nil {
			_, err = tagListing.CallPost("new", url.Values{"name": {tag}, "comment": {value}})
			if err != nil {
				return err
			}
			tagObj, err = tagListing.GetSubObject(tag).Get()
			if err != nil {
				return err
			}
		}
		_, err = tagObj.CallPost("update_nodes", url.Values{"add": {systemID}})
		if err != nil {
			return err
		}
	}
	return nil
}

func (m maasPlugin) deleteTagsFromNode(systemID string, tags []maas.MAASObject) error {
	for _, tag := range tags {
		_, err := tag.CallPost("update_nodes", url.Values{"remove": {systemID}})
		if err != nil {
			return err
		}
	}
	return nil

}

func (m maasPlugin) getTagsFromNode(systemID string) (map[string]string, error) {
	ret := map[string]string{}
	nodeListing := m.MaasObj.GetSubObject("nodes")
	listNodeObjects, err := nodeListing.CallGet("list", url.Values{})
	if err != nil {
		return nil, err
	}
	listNodes, err := listNodeObjects.GetArray()
	for _, nodeObj := range listNodes {
		node, err := nodeObj.GetMAASObject()
		if err != nil {
			return nil, err
		}
		id, err := node.GetField("system_id")
		if id == systemID {
			tags, err := node.GetMap()["tag_names"].GetArray()
			if err != nil {
				return nil, err
			}
			for _, tagObj := range tags {
				tag, err := tagObj.GetMAASObject()
				if err != nil {
					return nil, err
				}
				tagname, err := tag.GetField("name")
				if err != nil {
					return nil, err
				}
				tagcomment, err := tag.GetField("comment")
				if err != nil {
					return nil, err
				}
				ret[tagname] = tagcomment
			}
			return ret, nil
		}
	}
	return ret, nil
}

func (m maasPlugin) checkDuplicate(systemID string) (bool, error) {
	files, err := ioutil.ReadDir(m.MaasfilesDir)
	if err != nil {
		return false, err
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		machineDir := path.Join(m.MaasfilesDir, file.Name())
		hID, err := ioutil.ReadFile(path.Join(machineDir, "MachineID"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, err
		}
		if systemID == string(hID) {
			return true, nil
		}
	}
	return false, nil
}

// Provision creates a new instance.
func (m maasPlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	var properties map[string]interface{}
	if spec.Properties != nil {
		if err := spec.Properties.Decode(&properties); err != nil {
			return nil, fmt.Errorf("Invalid instance properties: %s", err)
		}
	}
	nodeListing := m.MaasObj.GetSubObject("nodes")
	jsonResponse, err := nodeListing.CallPost("acquire", m.convertSpecToMaasParam(properties))
	if err != nil {
		return nil, err
	}
	acquiredNode, err := jsonResponse.GetMAASObject()
	if err != nil {
		return nil, err
	}
	listNodeObjects, err := nodeListing.CallGet("list", url.Values{})
	if err != nil {
		return nil, err
	}
	listNodes, err := listNodeObjects.GetArray()
	if err != nil {
		return nil, err
	}
	notDuplicate := false
	for range listNodes {
		systemID, err := acquiredNode.GetField("system_id")
		if err != nil {
			return nil, err
		}
		isdup, err := m.checkDuplicate(systemID)
		if err != nil {
			return nil, err
		}
		if isdup {
			jsonResponse, err = nodeListing.CallPost("acquire", m.convertSpecToMaasParam(properties))
			if err != nil {
				return nil, err
			}
			acquiredNode, err = jsonResponse.GetMAASObject()
			if err != nil {
				return nil, err
			}
		} else {
			notDuplicate = true
			break
		}
	}
	if !notDuplicate {
		return nil, errors.New("Failed to aquire node")
	}
	isAcquired := make(chan bool)
	go func() {
		for state, err := acquiredNode.GetField("substatus_name"); state != "Allocated" && err == nil; state, err = acquiredNode.GetField("substatus_name") {
			time.Sleep(100)
		}
		isAcquired <- true
		return
	}()
	<-isAcquired
	params := url.Values{}
	if _, err = acquiredNode.CallPost("start", params); err != nil {
		return nil, err
	}
	systemID, err := acquiredNode.GetField("system_id")
	if err != nil {
		return nil, err
	}
	id := instance.ID(systemID)
	machineDir, err := ioutil.TempDir(m.MaasfilesDir, "infrakit-")
	if err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(path.Join(machineDir, "MachineID"), []byte(systemID), 0755); err != nil {
		return nil, err
	}
	err = m.addTagsToNode(systemID, spec.Tags)
	if err != nil {
		return nil, err
	}
	if spec.LogicalID != nil {
		if err := ioutil.WriteFile(path.Join(machineDir, "ip"), []byte(*spec.LogicalID), 0666); err != nil {
			return nil, err
		}
	}
	return &id, nil
}

// Label labels the instance
func (m maasPlugin) Label(id instance.ID, labels map[string]string) error {
	nodeListing := m.MaasObj.GetSubObject("nodes")
	listNodeObjects, err := nodeListing.CallGet("list", url.Values{})
	if err != nil {
		return err
	}
	listNodes, err := listNodeObjects.GetArray()
	for _, nodeObj := range listNodes {
		node, err := nodeObj.GetMAASObject()
		if err != nil {
			return err
		}
		systemID, err := node.GetField("system_id")
		if string(id) == systemID {
			tagObjs, err := node.GetMap()["tag_names"].GetArray()
			if err != nil {
				return err
			}
			tags := make([]maas.MAASObject, len(tagObjs))
			for i, tagObj := range tagObjs {
				tag, err := tagObj.GetMAASObject()
				if err != nil {
					return err
				}
				tags[i] = tag
			}

			m.deleteTagsFromNode(systemID, tags)
		}
	}
	return m.addTagsToNode(string(id), labels)
}

// Destroy terminates an existing instance.
func (m maasPlugin) Destroy(id instance.ID) error {
	fmt.Println("Destroying ", id)
	nodeListing := m.MaasObj.GetSubObject("nodes")
	listNodeObjects, err := nodeListing.CallGet("list", url.Values{})
	if err != nil {
		return err
	}
	listNodes, err := listNodeObjects.GetArray()
	for _, nodeObj := range listNodes {
		node, err := nodeObj.GetMAASObject()
		if err != nil {
			return err
		}
		systemID, err := node.GetField("system_id")
		if err != nil {
			return err
		}
		if systemID == string(id) {
			tagObjs, err := node.GetMap()["tag_names"].GetArray()
			tags := make([]maas.MAASObject, len(tagObjs))
			for i, tagObj := range tagObjs {
				tag, err := tagObj.GetMAASObject()
				if err != nil {
					return err
				}
				tags[i] = tag
			}
			m.deleteTagsFromNode(string(id), tags)
			if state, _ := node.GetField("substatus_name"); state == "Deploying" {
				params := url.Values{}
				if _, err = node.CallPost("abort_operation", params); err != nil {
					return err
				}
			}
			params := url.Values{}
			if _, err = node.CallPost("release", params); err != nil {
				return err
			}
		}
	}
	files, err := ioutil.ReadDir(m.MaasfilesDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		machineDir := path.Join(m.MaasfilesDir, file.Name())
		systemID, err := ioutil.ReadFile(path.Join(machineDir, "MachineID"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		} else if id == instance.ID(systemID) {
			if err := os.RemoveAll(machineDir); err != nil {
				return err
			}
		}
	}
	return nil
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (m maasPlugin) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
	var ret []instance.Description
	nodeListing := m.MaasObj.GetSubObject("nodes")
	listNodeObjects, err := nodeListing.CallGet("list", url.Values{})
	if err != nil {
		return nil, err
	}
	listNodes, err := listNodeObjects.GetArray()
	for _, nodeObj := range listNodes {
		node, err := nodeObj.GetMAASObject()
		if err != nil {
			return nil, err
		}
		nodeTags, err := node.GetMap()["tag_names"].GetArray()
		if err != nil {
			return nil, err
		}
		allMatched := true
		machineTags := make(map[string]string)
		for k, v := range tags {
			for _, tagObj := range nodeTags {
				tag, err := tagObj.GetMAASObject()
				if err != nil {
					return nil, err
				}
				tagname, err := tag.GetField("name")
				if err != nil {
					return nil, err
				}
				tagcomment, err := tag.GetField("comment")
				if err != nil {
					return nil, err
				}
				machineTags[tagname] = tagcomment
			}
			value, exists := machineTags[k]
			if !exists || v != value {
				allMatched = false
			}
		}
		if allMatched {
			systemID, err := node.GetField("system_id")
			if err != nil {
				return nil, err
			}
			ret = append(ret, instance.Description{
				ID:   instance.ID(systemID),
				Tags: machineTags,
			})
		}
	}
	return ret, nil
}

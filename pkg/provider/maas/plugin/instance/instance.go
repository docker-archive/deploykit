package instance

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	maas "github.com/juju/gomaasapi"
	"net/url"
)

// NewMaasPlugin creates an instance plugin for MaaS.
func NewMaasPlugin(dir string, key string, url string, version string) instance.Plugin {
	var err error
	var authClient *maas.Client
	url = url + "/MAAS"
	verurl := maas.AddAPIVersionToURL(url, version)
	if key != "" {
		authClient, err = maas.NewAuthenticatedClient(verurl, key)
	} else {
		authClient, err = maas.NewAnonymousClient(url, version)
	}
	if err != nil {
		return nil
	}
	ctl, err := maas.NewController(maas.ControllerArgs{
		BaseURL: verurl,
		APIKey:  key,
	})
	if err != nil {
		return nil
	}
	maasobj := maas.NewMAAS(*authClient)
	return &maasPlugin{MaasfilesDir: dir, MaasObj: maasobj, controller: ctl}
}

type maasPlugin struct {
	MaasfilesDir string
	MaasObj      *maas.MAASObject
	controller   maas.Controller
}

// Validate performs local validation on a provision request.
func (m maasPlugin) Validate(req *types.Any) error {
	return nil
}

func (m maasPlugin) convertSpecToMaasParam(spec map[string]interface{}) url.Values {
	param := url.Values{}
	return param
}

func (m maasPlugin) addTag(name string, comment string) (maas.JSONObject, error) {
	tl := m.MaasObj.GetSubObject("tags")
	t, err := tl.CallPost("", url.Values{"name": []string{name}, "comment": []string{comment}})
	return t, err
}

func (m maasPlugin) delTag(name string) error {
	err := m.MaasObj.GetSubObject("tags").GetSubObject(name).Delete()
	return err
}

func (m maasPlugin) addTagToNodes(systemIDs []string, tagname string, comment string) error {
	tObj, err := m.MaasObj.GetSubObject("tags").GetSubObject(tagname).Get()
	if err != nil {
		t, err := m.addTag(tagname, comment)
		if err != nil {
			return err
		}
		tObj, err = t.GetMAASObject()
		if err != nil {
			return err
		}
	}
	_, err = tObj.CallPost("update_nodes", url.Values{"add": systemIDs})
	if err != nil {
		return err
	}
	return nil
}

func (m maasPlugin) removeTagfromNodes(systemIDs []string, tag string) error {
	tObj := m.MaasObj.GetSubObject("tags").GetSubObject(tag)
	_, err := tObj.CallPost("update_nodes", url.Values{"remove": systemIDs})
	if err != nil {
		return err
	}
	return nil
}

func (m maasPlugin) getTagsFromNode(systemID string) ([]string, error) {
	ms, err := m.controller.Machines(maas.MachinesArgs{SystemIDs: []string{systemID}})
	if err != nil {
		return nil, err
	}
	if len(ms) != 1 {
		return nil, fmt.Errorf("Invalid systemID %s", systemID)
	}
	ret := ms[0].Tags()
	return ret, nil
}

// Provision creates a new instance.
func (m maasPlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	var properties map[string]interface{}
	if spec.Properties != nil {
		if err := spec.Properties.Decode(&properties); err != nil {
			return nil, fmt.Errorf("Invalid instance properties: %s", err)
		}
	}
	ama := maas.AllocateMachineArgs{}
	if spec.LogicalID != nil {
		ms, err := m.controller.Machines(maas.MachinesArgs{})
		if err != nil {
			return nil, err
		}
		ipcont := func(reqip string, machines []maas.Machine) (bool, string) {
			for _, i := range machines {
				if arrayContains(i.IPAddresses(), reqip) {
					return true, i.Hostname()
				}
			}
			return false, ""
		}
		r, hn := ipcont(string(*spec.LogicalID), ms)
		if !r {
			return nil, fmt.Errorf("Invalid LogicalID (%s) you should set static IP", *spec.LogicalID)
		}
		ama.Hostname = hn
	}
	am, _, err := m.controller.AllocateMachine(ama)
	if err != nil {
		return nil, err
	}
	if err := am.Start(maas.StartArgs{
		UserData: base64.StdEncoding.EncodeToString([]byte(spec.Init)),
	}); err != nil {
		return nil, err
	}
	systemID := am.SystemID()
	id := instance.ID(systemID)
	machineDir, err := ioutil.TempDir(m.MaasfilesDir, "infrakit-")
	if err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(path.Join(machineDir, "MachineID"), []byte(systemID), 0755); err != nil {
		return nil, err
	}
	err = m.Label(id, spec.Tags)
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
	tags, err := m.getTagsFromNode(string(id))
	if err != nil {
		return err
	}
	for _, t := range tags {
		m.removeTagfromNodes([]string{string(id)}, t)
	}
	for k, v := range labels {
		tag := strings.Replace(k, ".", "_", -1) + "_" + v
		err = m.addTagToNodes([]string{string(id)}, tag, v)
		if err != nil {
			return err
		}
	}
	return nil
}

// Destroy terminates an existing instance.
func (m maasPlugin) Destroy(id instance.ID, context instance.Context) error {
	err := m.Label(id, map[string]string{})
	if err != nil {
		return err
	}
	err = m.controller.ReleaseMachines(maas.ReleaseMachinesArgs{SystemIDs: []string{string(id)}})
	if err != nil {
		return err
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

func arrayContains(arr []string, str string) bool {
	for _, v := range arr {
		if v == str {
			return true
		}
	}
	return false
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (m maasPlugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	var ret []instance.Description
	ms, err := m.controller.Machines(maas.MachinesArgs{})
	if err != nil {
		return nil, err
	}
	tagcomp := func(reqtags map[string]string, nodetags []string) bool {
		for ot, v := range tags {
			tag := strings.Replace(ot, ".", "_", -1) + "_" + v
			if !arrayContains(nodetags, tag) {
				return false
			}
		}
		return true
	}
	for _, m := range ms {
		nodeTags := m.Tags()
		if err != nil {
			return nil, err
		}
		if tagcomp(tags, nodeTags) {
			systemID := m.SystemID()
			lid := instance.LogicalID(m.IPAddresses()[0])
			ret = append(ret, instance.Description{
				ID:        instance.ID(systemID),
				Tags:      tags,
				LogicalID: &lid,
			})
		}
	}
	return ret, nil
}

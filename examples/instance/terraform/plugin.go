package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/deckarep/golang-set"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/nightlyone/lockfile"
	"github.com/spf13/afero"
)

// This example uses terraform as the instance plugin.
// It is very similar to the file instance plugin.  When we
// provision an instance, we write a *.tf.json file in the directory
// and call terraform apply.  For describing instances, we parse the
// result of terraform show.  Destroying an instance is simply removing a
// tf.json file and call terraform apply again.

type plugin struct {
	Dir       string
	fs        afero.Fs
	lock      lockfile.Lockfile
	applying  bool
	applyLock sync.Mutex
	pretend   bool // true to actually do terraform apply
}

// NewTerraformInstancePlugin returns an instance plugin backed by disk files.
func NewTerraformInstancePlugin(dir string) instance.Plugin {
	log.Debugln("terraform instance plugin. dir=", dir)
	lock, err := lockfile.New(filepath.Join(dir, "tf-apply.lck"))
	if err != nil {
		panic(err)
	}

	return &plugin{
		Dir:  dir,
		fs:   afero.NewOsFs(),
		lock: lock,
	}
}

/*
TFormat models the on disk representation of a terraform resource JSON.

An example of this looks like:

{
    "resource" : {
	"aws_instance" : {
	    "web4" : {
		"ami" : "${lookup(var.aws_amis, var.aws_region)}",
		"instance_type" : "m1.small",
		"key_name": "PUBKEY",
		"vpc_security_group_ids" : ["${aws_security_group.default.id}"],
		"subnet_id": "${aws_subnet.default.id}",
		"tags" :  {
		    "Name" : "web4",
		    "InstancePlugin" : "terraform"
		}
		"connection" : {
		    "user" : "ubuntu"
		},
		"provisioner" : {
		    "remote_exec" : {
			"inline" : [
			    "sudo apt-get -y update",
			    "sudo apt-get -y install nginx",
			    "sudo service nginx start"
			]
		    }
		}
	    }
	}
    }
}

The block above is essentially embedded inside the `Properties` field of the instance Spec:

{
    "Properties" : {
      "resource" : {
    	 "aws_instance" : {
	    "web4" : {
		"ami" : "${lookup(var.aws_amis, var.aws_region)}",
		"instance_type" : "m1.small",
		"key_name": "PUBKEY",
		"vpc_security_group_ids" : ["${aws_security_group.default.id}"],
		"subnet_id": "${aws_subnet.default.id}",
		"tags" :  {
		    "Name" : "web4",
		    "InstancePlugin" : "terraform"
		}
		"connection" : {
		    "user" : "ubuntu"
		},
		"provisioner" : {
		    "remote_exec" : {
			"inline" : [
			    "sudo apt-get -y update",
			    "sudo apt-get -y install nginx",
			    "sudo service nginx start"
			]
		    }
		}
	    }
	 }
      }
    },
    "Tags" : {
        "other" : "values",
        "to" : "merge",
        "with" : "tags"
    },
    "Init" : "init string"
}

*/

// TResourceType is the type name of the resource: e.g. ibmcloud_infra_virtual_guest
type TResourceType string

// TResourceName is the name of the resource e.g. host1
type TResourceName string

// TResourceProperties is a dictionary for the resource
type TResourceProperties map[string]interface{}

// TFormat is the on-disk file format for the instance-xxxx.json.  This supports multiple resources.
type TFormat struct {
	// Resource matches the resource structure of the tf.json resource section
	Resource map[TResourceType]map[TResourceName]TResourceProperties `json:"resource"`
}

const (
	//VMAmazon is the resource type for aws
	VMAmazon = TResourceType("aws_instance")

	// VMAzure is the resource type for azure
	VMAzure = TResourceType("azurerm_virtual_machine")

	// VMDigitalOcean is the resource type for digital ocean
	VMDigitalOcean = TResourceType("digitalocean_droplet")

	// VMGoogleCloud is the resource type for google
	VMGoogleCloud = TResourceType("google_compute_instance")

	// VMSoftLayer is the resource type for softlayer
	VMSoftLayer = TResourceType("softlayer_virtual_guest")
)

var (
	// VMTypes is a list of supported vm types.
	VMTypes = []interface{}{VMAmazon, VMAzure, VMDigitalOcean, VMGoogleCloud, VMSoftLayer}
)

// first returns the first entry.  This is based on our assumption that exactly one vm resource per file.
func first(vms map[TResourceName]TResourceProperties) (TResourceName, TResourceProperties) {
	for k, v := range vms {
		return k, v
	}
	return TResourceName(""), nil
}

// FindVM finds the resource block representing the vm instance from the tf.json representation
func FindVM(tf *TFormat) (vmType TResourceType, vmName TResourceName, properties TResourceProperties, err error) {
	if tf.Resource == nil {
		err = fmt.Errorf("no resource section")
		return
	}

	supported := mapset.NewSetFromSlice(VMTypes)
	for resourceType, objs := range tf.Resource {
		if supported.Contains(resourceType) {
			vmType = resourceType
			vmName, properties = first(objs)
			return
		}
	}
	err = fmt.Errorf("not found")
	return
}

// Validate performs local validation on a provision request.
func (p *plugin) Validate(req *types.Any) error {
	log.Debugln("validate", req.String())

	tf := TFormat{}
	err := req.Decode(&tf)
	if err != nil {
		return err
	}

	vmTypes := mapset.NewSetFromSlice(VMTypes)
	vms := 0
	for k := range tf.Resource {
		if vmTypes.Contains(k) {
			vms++
		}
	}

	if vms > 1 {
		return fmt.Errorf("zero or 1 vm instance per request: %d", vms)
	}

	return nil
}

func addUserData(m map[string]interface{}, key string, init string) {
	if v, has := m[key]; has {
		m[key] = fmt.Sprintf("%s\n%s", v, init)
	} else {
		m[key] = init
	}
}

func (p *plugin) scanLocalFiles() (map[TResourceType]map[TResourceName]TResourceProperties, error) {
	re := regexp.MustCompile("(^instance-[0-9]+)(.tf.json)")

	vms := map[TResourceType]map[TResourceName]TResourceProperties{}

	fs := &afero.Afero{Fs: p.fs}
	// just scan the directory for the instance-*.tf.json files
	err := fs.Walk(p.Dir,

		func(path string, info os.FileInfo, err error) error {
			matches := re.FindStringSubmatch(info.Name())

			if len(matches) == 3 {
				buff, err := ioutil.ReadFile(filepath.Join(p.Dir, info.Name()))
				if err != nil {
					log.Warningln("Cannot parse:", err)
					return err
				}
				tf := TFormat{}
				if err = types.AnyBytes(buff).Decode(&tf); err != nil {
					return err
				}
				vmType, vmName, props, err := FindVM(&tf)
				if err != nil {
					return err
				}

				if _, has := vms[vmType]; !has {
					vms[vmType] = map[TResourceName]TResourceProperties{}
				}
				vms[vmType][vmName] = props
			}
			return nil
		})
	return vms, err
}

func (p *plugin) ensureUniqueFile() string {
	for {
		if err := p.lock.TryLock(); err == nil {
			defer p.lock.Unlock()
			return ensureUniqueFile(p.Dir)
		}
		log.Infoln("Can't acquire lock, waiting")
		time.Sleep(time.Duration(int64(rand.NormFloat64())%1000) * time.Millisecond)
	}
}

func ensureUniqueFile(dir string) string {
	n := fmt.Sprintf("instance-%d", time.Now().Unix())
	// if we can open then we have to try again... the file cannot exist currently
	if f, err := os.Open(filepath.Join(dir, n) + ".tf.json"); err == nil {
		f.Close()
		return ensureUniqueFile(dir)
	}
	return n
}

// Special processing of hostname on some platforms. Where supported, you can
// add a special @hostname_prefix that will allow the setting of hostname in given format
// TODO - expand this to formatting string
func (p *plugin) optionalProcessHostname(vmType TResourceType, name TResourceName, properties TResourceProperties) {

	if properties == nil {
		return
	}

	switch vmType {
	case TResourceType("softlayer_virtual_guest"): // # list the platforms here
	default:
		return
	}

	// Use the given hostname value as a prefix if it is a non-empty string
	if hostnamePrefix, is := properties["@hostname_prefix"].(string); is {
		hostnamePrefix = strings.Trim(hostnamePrefix, " ")
		// Use the default behavior if hostnamePrefix was either not a string, or an empty string
		if hostnamePrefix == "" {
			properties["hostname"] = string(name)
		} else {
			// Remove "instance-" from "instance-XXXX", then append that string to the hostnamePrefix to create the new hostname
			properties["hostname"] = fmt.Sprintf("%s-%s", hostnamePrefix, strings.Replace(string(name), "instance-", "", -1))
		}
	} else {
		properties["hostname"] = name
	}
	// Delete hostnamePrefix so it will not be written in the *.tf.json file
	delete(properties, "@hostname_prefix")
	log.Debugln("Adding hostname to properties: hostname=", properties["hostname"])
}

// Provision creates a new instance based on the spec.
func (p *plugin) Provision(spec instance.Spec) (*instance.ID, error) {

	// Because the format of the spec.Properties is simply the same tf.json
	// we simply look for vm instance and merge in the tags, and user init, etc.

	tf := TFormat{}
	err := spec.Properties.Decode(&tf)
	if err != nil {
		return nil, err
	}

	vmType, vmName, properties, err := FindVM(&tf)
	if err != nil {
		return nil, err
	}

	if properties == nil {
		return nil, fmt.Errorf("no-vm-instance-in-spec")
	}

	// use timestamp as instance id
	name := p.ensureUniqueFile()

	id := instance.ID(name)

	// set the tags.
	// add a name
	if spec.Tags != nil {
		switch vmType {
		case VMSoftLayer:
			// Set the "name" tag to be lowercase to meet platform requirements
			if _, has := spec.Tags["name"]; !has {
				spec.Tags["name"] = string(id)
			}
		default:
			// Set the first character of the "Name" tag to be uppercase to meet platform requirements
			if _, has := spec.Tags["Name"]; !has {
				spec.Tags["Name"] = string(id)
			}
		}
	}

	p.optionalProcessHostname(vmType, TResourceName(name), properties)

	switch vmType {
	case VMAmazon, VMAzure, VMDigitalOcean, VMGoogleCloud:
		if t, exists := properties["tags"]; !exists {
			properties["tags"] = spec.Tags
		} else if mm, ok := t.(map[string]interface{}); ok {
			// merge tags
			for tt, vv := range spec.Tags {
				mm[tt] = vv
			}
		}
	case VMSoftLayer:
		if _, has := properties["tags"]; !has {
			properties["tags"] = []interface{}{}
		}
		tags, ok := properties["tags"].([]interface{})
		if ok {
			//softlayer uses a list of tags, instead of a map of tags
			properties["tags"] = mergeLabelsIntoTagSlice(tags, spec.Tags)
		}
	}

	// Use tag to store the logical id
	if spec.LogicalID != nil {
		if m, ok := properties["tags"].(map[string]interface{}); ok {
			m["LogicalID"] = string(*spec.LogicalID)
		}
	}
	switch vmType {
	case VMAmazon:
		if p, exists := properties["private_ip"]; exists {
			if p == "INSTANCE_LOGICAL_ID" {
				if spec.LogicalID != nil {
					// set private IP to logical ID
					properties["private_ip"] = string(*spec.LogicalID)
				} else {
					// reset private IP (the tag is not relevant in this context)
					delete(properties, "private_ip")
				}
			}
		}
	}

	// merge the inits
	switch vmType {
	case VMAmazon, VMDigitalOcean:
		addUserData(properties, "user_data", base64.StdEncoding.EncodeToString([]byte(spec.Init)))
	case VMSoftLayer:
		addUserData(properties, "user_metadata", spec.Init)
	case VMAzure:
		// os_profile.custom_data
		if m, has := properties["os_profile"]; !has {
			properties["os_profile"] = map[string]interface{}{
				"custom_data": spec.Init,
			}
		} else if mm, ok := m.(map[string]interface{}); ok {
			addUserData(mm, "custom_data", spec.Init)
		}
	case VMGoogleCloud:
		// metadata_startup_script
		addUserData(properties, "metadata_startup_script", spec.Init)
	}

	// Write the whole thing back out, after decorations and replacing the hostname with the generated hostname
	delete(tf.Resource[vmType], vmName)
	tf.Resource[vmType][TResourceName(name)] = properties

	buff, err := json.MarshalIndent(tf, "  ", "  ")
	log.Debugln("provision", id, "data=", string(buff), "err=", err)
	if err != nil {
		return nil, err
	}

	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, string(id)+".tf.json"), buff, 0644)
	if err != nil {
		return nil, err
	}

	return &id, p.terraformApply()
}

// Label labels the instance
func (p *plugin) Label(instance instance.ID, labels map[string]string) error {
	buff, err := afero.ReadFile(p.fs, filepath.Join(p.Dir, string(instance)+".tf.json"))
	if err != nil {
		return err
	}

	tf := TFormat{}
	err = types.AnyBytes(buff).Decode(&tf)
	if err != nil {
		return err
	}

	vmType, vmName, props, err := FindVM(&tf)
	if err != nil {
		return err
	}

	if len(props) == 0 || vmName != TResourceName(string(instance)) {
		return fmt.Errorf("not found:%v", instance)
	}

	switch vmType {
	case VMAmazon, VMAzure, VMDigitalOcean, VMGoogleCloud:
		if _, has := props["tags"]; !has {
			props["tags"] = map[string]interface{}{}
		}

		if tags, ok := props["tags"].(map[string]interface{}); ok {
			for k, v := range labels {
				tags[k] = v
			}
		}

	case VMSoftLayer:
		if _, has := props["tags"]; !has {
			props["tags"] = []interface{}{}
		}
		tags, ok := props["tags"].([]interface{})
		if !ok {
			return fmt.Errorf("bad format:%v", instance)
		}
		props["tags"] = mergeLabelsIntoTagSlice(tags, labels)
	}

	buff, err = json.MarshalIndent(tf, "  ", "  ")
	if err != nil {
		return err
	}
	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, string(instance)+".tf.json"), buff, 0644)
	if err != nil {
		return err
	}
	return p.terraformApply()
}

// Destroy terminates an existing instance.
func (p *plugin) Destroy(instance instance.ID) error {
	fp := filepath.Join(p.Dir, string(instance)+".tf.json")
	log.Debugln("destroy instance", fp)
	err := p.fs.Remove(fp)
	if err != nil {
		return err
	}
	return p.terraformApply()
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (p *plugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	log.Debugln("describe-instances", tags)

	// localSpecs are what we told terraform to create - these are the generated files.
	localSpecs, err := p.scanLocalFiles()
	if err != nil {
		return nil, err
	}

	terraformShowResult := map[TResourceType]map[TResourceName]TResourceProperties{}
	if properties {
		// TODO - not the most efficient, but here we assume we're usually just one vm type
		for vmResourceType := range localSpecs {

			if instances, err := doTerraformShow(p.Dir, vmResourceType); err == nil {

				terraformShowResult[vmResourceType] = instances

			} else {
				// Don't blow up... just do best and show what we can find.
				log.Warnln("cannot terraform show:", err)
			}
		}
	}

	re := regexp.MustCompile("(.*)(instance-[0-9]+)")
	result := []instance.Description{}
	// now we scan for <instance_type.instance-<timestamp> as keys
scan:
	for t, vm := range localSpecs {

		for k, v := range vm {
			matches := re.FindStringSubmatch(string(k))
			if len(matches) == 3 {
				id := matches[2]

				inst := instance.Description{
					Tags:      terraformTags(v, "tags"),
					ID:        instance.ID(id),
					LogicalID: terraformLogicalID(v),
				}

				if properties {
					if vms, has := terraformShowResult[t]; has {
						if details, has := vms[k]; has {

							if encoded, err := types.AnyValue(details); err == nil {
								inst.Properties = encoded
							}

						}
					}
				}

				if len(tags) == 0 {
					result = append(result, inst)
				} else {
					for k, v := range tags {
						if inst.Tags[k] != v {
							continue scan // we implement AND
						}
					}
					result = append(result, inst)
				}
			}
		}

	}

	log.Debugln("describe-instances result=", result)

	return result, nil
}

func terraformTags(m TResourceProperties, key string) map[string]string {
	tags := map[string]string{}
	if mm, ok := m[key].(map[string]interface{}); ok {
		for k, v := range mm {
			tags[k] = fmt.Sprintf("%v", v)
		}
		return tags
	} else if mm, ok := m[key].([]interface{}); ok {
		// add each tag in the list to the tags map
		for _, v := range mm {
			value := fmt.Sprintf("%v", v)
			if strings.Contains(value, ":") {
				log.Debugln("terraformTags system tags detected v=", v)
				// This assumes that the first colon is separating the key and the value of the tag.
				// This is done so that colons are valid characters in the value.
				vv := strings.SplitN(value, ":", 2)
				if len(vv) == 2 {
					tags[vv[0]] = vv[1]
				} else {
					log.Errorln("terraformTags: ignore invalid tag detected", value)
				}
			} else {
				tags[value] = "" // for list but no ':"
			}
		}
		log.Debugln("terraformTags return tags", tags)
		return tags
	} else {
		log.Errorln("terraformTags: invalid terraformTags tags value", m[key])
	}

	for k, v := range m {
		if k != "tags.%" && strings.Index(k, "tags.") == 0 {
			n := k[len("tags."):]
			tags[n] = fmt.Sprintf("%v", v)
		}
	}
	return tags
}
func terraformLogicalID(v interface{}) *instance.LogicalID {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	tags, ok := m["tags"].(map[string]interface{})
	if !ok {
		return nil
	}
	v, exists := tags["LogicalID"]
	if exists {
		id := instance.LogicalID(fmt.Sprintf("%v", v))
		return &id
	}
	return nil
}

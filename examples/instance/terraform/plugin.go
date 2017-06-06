package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/deckarep/golang-set"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
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
	"resource": {
		"aws_instance": {
			"web4": {
				"ami": "${lookup(var.aws_amis, var.aws_region)}",
				"instance_type": "m1.small",
				"key_name": "PUBKEY",
				"vpc_security_group_ids": ["${aws_security_group.default.id}"],
				"subnet_id": "${aws_subnet.default.id}",
				"tags":  {
					"Name": "web4",
					"InstancePlugin": "terraform"
				},
				"connection": {
					"user": "ubuntu"
				},
				"provisioner": {
					"remote_exec": {
						"inline": [
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
	"Properties": {
		"resource": {
			"aws_instance": {
				"web4": {
					"ami": "${lookup(var.aws_amis, var.aws_region)}",
					"instance_type": "m1.small",
					"key_name": "PUBKEY",
					"vpc_security_group_ids": ["${aws_security_group.default.id}"],
					"subnet_id": "${aws_subnet.default.id}",
					"tags":  {
						"Name": "web4",
						"InstancePlugin": "terraform"
					},
					"connection": {
						"user": "ubuntu"
					},
					"provisioner": {
						"remote_exec": {
							"inline": [
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
	"Tags": {
		"other": "values",
		"to": "merge",
		"with": "tags"
	},
	"Init": "init string"
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

// platformSpecificUpdates handles unique platform specific logic
func platformSpecificUpdates(vmType TResourceType, name TResourceName, logicalID *instance.LogicalID, properties TResourceProperties) {
	if properties == nil {
		return
	}
	switch vmType {
	case VMSoftLayer:
		// Process a special @hostname_prefix that will allow the setting of hostname in a
		// specific format; use the LogicalID (if set), else the name
		var hostname string
		if logicalID == nil {
			hostname = string(name)
		} else {
			hostname = string(*logicalID)
		}
		// Use the given hostname value as a prefix if it is a non-empty string
		if hostnamePrefix, is := properties["@hostname_prefix"].(string); is {
			hostnamePrefix = strings.Trim(hostnamePrefix, " ")
			// Use the default behavior if hostnamePrefix was either not a string, or an empty string
			if hostnamePrefix == "" {
				properties["hostname"] = hostname
			} else {
				// Remove "instance-" from "instance-XXXX", then append that string to the hostnamePrefix to create the new hostname
				properties["hostname"] = fmt.Sprintf("%s-%s", hostnamePrefix, strings.Replace(hostname, "instance-", "", -1))
			}
		} else {
			properties["hostname"] = hostname
		}
		// Delete hostnamePrefix so it will not be written in the *.tf.json file
		delete(properties, "@hostname_prefix")
		log.Debugln("Adding hostname to properties: hostname=", properties["hostname"])
	case VMAmazon:
		if p, exists := properties["private_ip"]; exists {
			if p == "INSTANCE_LOGICAL_ID" {
				if logicalID != nil {
					// set private IP to logical ID
					properties["private_ip"] = string(*logicalID)
				} else {
					// reset private IP (the tag is not relevant in this context)
					delete(properties, "private_ip")
				}
			}
		}
	}
}

// addUserData adds the given init data to the given map at the given key
func addUserData(m map[string]interface{}, key string, init string) {
	if v, has := m[key]; has {
		m[key] = fmt.Sprintf("%s\n%s", v, init)
	} else {
		m[key] = init
	}
}

// mergeInitScript merges the user defined user data with the spec init data
func mergeInitScript(spec instance.Spec, id instance.ID, vmType TResourceType, properties TResourceProperties) {
	// Merge the init scripts
	switch vmType {
	case VMAmazon, VMDigitalOcean:
		addUserData(properties, "user_data", spec.Init)
		properties["user_data"] = base64.StdEncoding.EncodeToString([]byte(properties["user_data"].(string)))
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
}

// renderInstIdVar applies the "/self/instId" as a global option and renders
// the given string
func renderInstIDVar(data string, id instance.ID) (string, error) {
	t, _ := template.NewTemplate("str://"+data, template.Options{})
	return t.Global("/self/instId", id).Render(nil)
}

// handleProvisionTags sets the Infrakit-specific tags and merges with the user-defined in the instance spec
func handleProvisionTags(spec instance.Spec, id instance.ID, vmType TResourceType, vmProperties TResourceProperties) {
	// Add the name to the tags if it does not exist, issue case-insensitive
	// check for the "name" key
	if spec.Tags != nil {
		match := false
		for key := range spec.Tags {
			if strings.ToLower(key) == "name" {
				match = true
				break
			}
		}
		if !match {
			spec.Tags["Name"] = string(id)
		}
	}
	// Use tag to store the logical id
	if spec.LogicalID != nil {
		spec.Tags["LogicalID"] = string(*spec.LogicalID)
	}

	// Merge any spec tags into the VM properties
	mergeTagsIntoVMProps(vmType, vmProperties, spec.Tags)
}

// mergeTagsIntoVMProps merges the given tags into vmProperties in the appropriate
// platform-specific tag format
func mergeTagsIntoVMProps(vmType TResourceType, vmProperties TResourceProperties, tags map[string]string) {
	switch vmType {
	case VMAmazon, VMAzure, VMDigitalOcean, VMGoogleCloud:
		if vmTags, exists := vmProperties["tags"]; !exists {
			// Need to be careful with type here; the tags saved in the VM properties need to be generic
			// since that it how they are parsed from json
			tagsInterface := make(map[string]interface{}, len(tags))
			for k, v := range tags {
				tagsInterface[k] = v
			}
			vmProperties["tags"] = tagsInterface
		} else if tagsMap, ok := vmTags.(map[string]interface{}); ok {
			// merge tags
			for k, v := range tags {
				tagsMap[k] = v
			}
		} else {
			log.Errorf("mergeTagsIntoVMProps: invalid %v props tags value: %v", vmType, reflect.TypeOf(vmProperties["tags"]))
		}
	case VMSoftLayer:
		if _, has := vmProperties["tags"]; !has {
			vmProperties["tags"] = []interface{}{}
		}
		if tagsArray, ok := vmProperties["tags"].([]interface{}); ok {
			// softlayer uses a list of tags, instead of a map of tags
			vmProperties["tags"] = mergeLabelsIntoTagSlice(tagsArray, tags)
		} else {
			log.Errorf("mergeTagsIntoVMProps: invalid %v props tags value: %v", vmType, reflect.TypeOf(vmProperties["tags"]))
		}
		// All tags on Softlayer must be lower-case
		tagsLower := []interface{}{}
		for _, val := range vmProperties["tags"].([]string) {
			tagsLower = append(tagsLower, strings.ToLower(val))
		}
		vmProperties["tags"] = tagsLower
	}
}

// writeTerraformFiles uses the data in the TFormat to create a .tf.json file with the
// generated name. The properties of the VM resource are overriden by vmProperties.
func (p *plugin) writeTerraformFiles(logicalID *instance.LogicalID, generatedName string, tf *TFormat, vmType TResourceType, vmProperties TResourceProperties) error {
	for resourceType, resourceObj := range tf.Resource {
		vmList := mapset.NewSetFromSlice(VMTypes)
		for resourceName, resourceProps := range resourceObj {
			var newResourceName string
			if vmList.Contains(resourceType) {
				// Overwrite with the changes to the VM properties
				resourceProps = vmProperties
				newResourceName = generatedName
			} else {
				newResourceName = fmt.Sprintf("%s-%s", generatedName, resourceName)
			}
			delete(tf.Resource[resourceType], resourceName)
			tf.Resource[resourceType][TResourceName(newResourceName)] = resourceProps
		}
	}
	// Handle any platform specific updates to the VM properties prior to writing out
	platformSpecificUpdates(vmType, TResourceName(generatedName), logicalID, vmProperties)

	buff, err := json.MarshalIndent(tf, "  ", "  ")
	log.Debugln("writeTerraformFiles", generatedName, "data=", string(buff), "err=", err)
	if err != nil {
		return err
	}
	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, generatedName+".tf.json"), buff, 0644)
	if err != nil {
		return err
	}
	return nil
}

// Provision creates a new instance based on the spec.
func (p *plugin) Provision(spec instance.Spec) (*instance.ID, error) {

	// Because the format of the spec.Properties is simply the same tf.json
	// we simply look for vm instance and merge in the tags, and user init, etc.

	// use timestamp as instance id
	name := p.ensureUniqueFile()
	id := instance.ID(name)

	// Template the {{ var "/self/instId" }} var in both the Properties and the Init
	rendered, err := renderInstIDVar(spec.Properties.String(), id)
	if err != nil {
		return nil, err
	}
	spec.Properties = types.AnyBytes([]byte(rendered))
	rendered, err = renderInstIDVar(spec.Init, id)
	if err != nil {
		return nil, err
	}
	spec.Init = rendered

	// Decode the given spec and find the VM resource
	tf := TFormat{}
	err = spec.Properties.Decode(&tf)
	if err != nil {
		return nil, err
	}
	vmType, _, vmProps, err := FindVM(&tf)
	if err != nil {
		return nil, err
	}
	if vmProps == nil {
		return nil, fmt.Errorf("no-vm-instance-in-spec")
	}

	// Add Infrakit-specific tags to the user-defined VM properties
	handleProvisionTags(spec, id, vmType, vmProps)
	// Merge the init scripts into the VM properties
	mergeInitScript(spec, id, vmType, vmProps)
	// Write out the tf.json file
	if err = p.writeTerraformFiles(spec.LogicalID, name, &tf, vmType, vmProps); err != nil {
		return nil, err
	}
	// And apply the updates
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

	vmType, vmName, vmProps, err := FindVM(&tf)
	if err != nil {
		return err
	}

	if len(vmProps) == 0 || vmName != TResourceName(string(instance)) {
		return fmt.Errorf("not found:%v", instance)
	}

	mergeTagsIntoVMProps(vmType, vmProps, labels)

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
	for t, vm := range localSpecs {
	scan:
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
				tags[vv[0]] = vv[1]
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

// terraformLogicalID parses the LogicalID (case insensitive key check) from
// either the map of tags or the list of tags
func terraformLogicalID(props TResourceProperties) *instance.LogicalID {
	if propsTag, ok := props["tags"]; ok {
		if tagsMap, ok := propsTag.(map[string]interface{}); ok {
			for key, val := range tagsMap {
				if strings.ToLower(key) == "logicalid" {
					id := instance.LogicalID(fmt.Sprintf("%v", val))
					return &id
				}
			}
		} else if tagsList, ok := propsTag.([]interface{}); ok {
			for _, tag := range tagsList {
				if tagString, ok := tag.(string); ok {
					if strings.HasPrefix(strings.ToLower(tagString), "logicalid:") {
						logicalID := strings.SplitN(strings.ToLower(tagString), ":", 2)[1]
						id := instance.LogicalID(fmt.Sprintf("%v", logicalID))
						return &id
					}
				}
			}
		}
	}
	return nil
}

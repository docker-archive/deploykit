package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/deckarep/golang-set"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/exec"
	"github.com/nightlyone/lockfile"
	"github.com/spf13/afero"
)

// This example uses terraform as the instance plugin.
// It is very similar to the file instance plugin.  When we
// provision an instance, we write a *.tf.json file in the directory
// and call terraform apply.  For describing instances, we parse the
// result of terraform show.  Destroying an instance is simply removing a
// tf.json file and call terraform apply again.

const (
	// AttachTag contains a space separated list of IDs that are to the instance
	attachTag = "infrakit.attach"
)

type plugin struct {
	Dir          string
	fs           afero.Fs
	fsLock       lockfile.Lockfile
	applying     bool
	applyLock    sync.Mutex
	pretend      bool // true to actually do terraform apply
	pollInterval time.Duration
	pollChannel  chan bool
	pluginLookup func() discovery.Plugins
}

// NewTerraformInstancePlugin returns an instance plugin backed by disk files.
func NewTerraformInstancePlugin(dir string, pollInterval time.Duration, standalone bool, bootstrapGrpSpecStr, bootstrapInstID string) instance.Plugin {
	log.Debugln("terraform instance plugin. dir=", dir)
	fsLock, err := lockfile.New(filepath.Join(dir, "tf-apply.lck"))
	if err != nil {
		panic(err)
	}

	var pluginLookup func() discovery.Plugins
	if !standalone {
		if err = local.Setup(); err != nil {
			panic(err)
		}
		plugins, err := local.NewPluginDiscovery()
		if err != nil {
			panic(err)
		}
		pluginLookup = func() discovery.Plugins {
			return plugins
		}
	}

	p := plugin{
		Dir:          dir,
		fs:           afero.NewOsFs(),
		fsLock:       fsLock,
		pollInterval: pollInterval,
		pluginLookup: pluginLookup,
	}

	// Handle bootstrap data
	var bootstrapGrpSpec group.Spec
	if bootstrapGrpSpecStr == "" {
		if bootstrapInstID != "" {
			panic(fmt.Errorf("Bootstrap group spec required with bootstrap instance ID"))
		}
	} else {
		if bootstrapInstID == "" {
			panic(fmt.Errorf("Bootstrap instance ID required with bootstrap group spec"))
		}
		t, err := template.NewTemplate(bootstrapGrpSpecStr, template.Options{MultiPass: false})
		if err != nil {
			panic(err)
		}
		template, err := t.Render(nil)
		if err != nil {
			panic(err)
		}
		if err = types.AnyString(template).Decode(&bootstrapGrpSpec); err != nil {
			panic(err)
		}
		if id, err := p.importResource(bootstrapInstID, bootstrapGrpSpec, true); err == nil {
			log.Infof("Successfull imported bootstrap instance %v with id %v", *id, bootstrapInstID)
		} else {
			log.Errorf("Failed to import bootstrap instance %v, error: %v", bootstrapInstID, err)
			panic(err)
		}
	}
	return &p
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

	// VMIBMCloud is the resource type for IBM Cloud
	VMIBMCloud = TResourceType("ibm_compute_vm_instance")
)

const (
	// PropHostnamePrefix is the optional terraform property that contains the hostname prefix
	PropHostnamePrefix = "@hostname_prefix"

	// PropScope is the optional terraform property that defines how a resource should be persisted
	PropScope = "@scope"

	// ValScopeDedicated defines dedicated scope: the resource lifecycle is loosely couple with the
	// VM; it is written in a file named "instance-xxxx-dedicated.tf.json"
	ValScopeDedicated = "@dedicated"

	// ValScopeDefault defines the default scope: the resource lifecycle is tightly coupled
	// with the VM; it is written in the same "instance.xxxx.tf.json" file
	ValScopeDefault = "@default"
)

var (
	// VMTypes is a list of supported vm types.
	VMTypes = []interface{}{VMAmazon, VMAzure, VMDigitalOcean, VMGoogleCloud, VMSoftLayer, VMIBMCloud}
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

// scanLocalFiles reads the filesystem and loads as tf.json files
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

// platformSpecificUpdates handles unique platform specific logic
func platformSpecificUpdates(vmType TResourceType, name TResourceName, logicalID *instance.LogicalID, properties TResourceProperties) {
	if properties == nil {
		return
	}
	switch vmType {
	case VMSoftLayer, VMIBMCloud:
		// Process a special @hostname_prefix that will allow the setting of hostname in a
		// specific format; use the LogicalID (if set), else the name
		var hostname string
		if logicalID == nil {
			hostname = string(name)
		} else {
			hostname = string(*logicalID)
		}
		// Use the given hostname value as a prefix if it is a non-empty string
		if hostnamePrefix, is := properties[PropHostnamePrefix].(string); is {
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
		delete(properties, PropHostnamePrefix)
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
	case VMSoftLayer, VMIBMCloud:
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
	case VMSoftLayer, VMIBMCloud:
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
			// Commas are not valid tag characters, change to a space for tag values that
			// are a list
			if strings.HasPrefix(val, attachTag+":") {
				val = strings.Replace(val, ",", " ", -1)
			}
			tagsLower = append(tagsLower, strings.ToLower(val))
		}
		vmProperties["tags"] = tagsLower
	}
}

// writeTerraformFiles uses the data in the TFormat to create one or more .tf.json files with the
// generated name. The properties of the VM resource are overriden by vmProperties.
func (p *plugin) writeTerraformFiles(logicalID *instance.LogicalID, generatedName string, tf *TFormat, vmType TResourceType, vmProperties TResourceProperties) error {
	// Map file names to the data in each file based on the "@scope" property:
	// - @default: resources in same "instance-xxxx.tf.json" file as VM
	// - @dedicated: resources in different file as VM with the same ID (instance-xxxx-dedicated.tf.json)
	// - <other>: resource defined in different file with a scope identifier (scope-<other>.tf.json)
	fileMap := make(map[string]*TFormat)

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
			// Determine the scope value (default to 'default')
			scope, has := resourceProps[PropScope]
			if has {
				delete(resourceProps, PropScope)
			} else {
				scope = ValScopeDefault
			}
			// Determine the filename based off of the scope value
			var filename string
			switch scope {
			case ValScopeDefault:
				filename = generatedName
			case ValScopeDedicated:
				filename = fmt.Sprintf("%s-dedicated", generatedName)
			default:
				filename = fmt.Sprintf("scope-%s", scope)
				// If the scope is global use it as the prefix for the resource name
				newResourceName = fmt.Sprintf("%s-%s", scope, resourceName)
			}
			// Get the associated value in the file map
			tfPersistence, has := fileMap[filename]
			if !has {
				resourceMap := make(map[TResourceType]map[TResourceName]TResourceProperties)
				tfPersistence = &TFormat{Resource: resourceMap}
				fileMap[filename] = tfPersistence
			}
			resourceNameMap, has := tfPersistence.Resource[resourceType]
			if !has {
				resourceNameMap = make(map[TResourceName]TResourceProperties)
				tfPersistence.Resource[resourceType] = resourceNameMap
			}
			resourceNameMap[TResourceName(newResourceName)] = resourceProps
		}
	}
	// Update the vmProperties with the infrakit.attach tag
	attach := []string{}
	for filename := range fileMap {
		if filename == generatedName {
			continue
		}
		attach = append(attach, filename)
	}
	if len(attach) > 0 {
		sort.Strings(attach)
		tags := map[string]string{
			attachTag: strings.Join(attach, ","),
		}
		mergeTagsIntoVMProps(vmType, vmProperties, tags)
	}
	// Handle any platform specific updates to the VM properties prior to writing out
	platformSpecificUpdates(vmType, TResourceName(generatedName), logicalID, vmProperties)

	for filename, tfVal := range fileMap {
		buff, err := json.MarshalIndent(tfVal, "  ", "  ")
		log.Debugln("writeTerraformFiles", filename, "data=", string(buff), "err=", err)
		if err != nil {
			return err
		}
		err = afero.WriteFile(p.fs, filepath.Join(p.Dir, filename+".tf.json"), buff, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

// ensureUniqueFile returns a filename that is not in use
func ensureUniqueFile(dir string) string {
	// use timestamp as instance id
	n := fmt.Sprintf("instance-%d", time.Now().Unix())
	// if we can open then we have to try again... the file cannot exist currently
	if f, err := os.Open(filepath.Join(dir, n) + ".tf.json"); err == nil {
		f.Close()
		return ensureUniqueFile(dir)
	}
	return n
}

// Provision creates a new instance based on the spec.
func (p *plugin) Provision(spec instance.Spec) (*instance.ID, error) {

	// Because the format of the spec.Properties is simply the same tf.json
	// we simply look for vm instance and merge in the tags, and user init, etc.

	// Hold the fs lock for the duration since the file is written at the end
	var name string
	for {
		if err := p.fsLock.TryLock(); err == nil {
			defer p.fsLock.Unlock()
			name = ensureUniqueFile(p.Dir)
			break
		}
		log.Infoln("Can't acquire fsLock on Provision, waiting")
		time.Sleep(time.Second)
	}
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
	// Acquire lock
	for {
		if err := p.fsLock.TryLock(); err == nil {
			defer p.fsLock.Unlock()
			break
		}
		time.Sleep(time.Second)
	}

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
func (p *plugin) Destroy(instance instance.ID, context instance.Context) error {
	// TODO(kaufers): When SPI is updated with context, updated 2nd arg to false
	// for rolling updated so that attached resources are not destroyed
	err := p.doDestroy(instance, true)
	return err
}

// doDestroy terminates an existing instance and optionally terminates any related
// resources
func (p *plugin) doDestroy(inst instance.ID, processAttach bool) error {
	// Acquire lock
	for {
		if err := p.fsLock.TryLock(); err == nil {
			defer p.fsLock.Unlock()
			break
		}
		time.Sleep(time.Second)
	}

	filename := string(inst) + ".tf.json"
	fp := filepath.Join(p.Dir, filename)

	log.Debugln("destroy instance", fp)
	// Optionally destroy the related resources
	if processAttach {
		// Get an referenced resources in the "infrakit.attach" tag
		attachIDs, err := parseAttachTagFromFile(fp)
		if err != nil {
			return err
		}
		if len(attachIDs) > 0 {
			idsToDestroy := make(map[string]string)
			for _, attachID := range attachIDs {
				idsToDestroy[attachID] = ""
			}
			// Load all other instance files and determine other references exist
			re := regexp.MustCompile("(^instance-[0-9]+)(.tf.json)")
			fs := &afero.Afero{Fs: p.fs}
			err := fs.Walk(p.Dir,
				func(path string, info os.FileInfo, err error) error {
					matches := re.FindStringSubmatch(info.Name())
					// Note that the current instance (being destroyed) still exists; filter
					// this file out.
					if len(matches) == 3 && filename != info.Name() {
						ids, err := parseAttachTagFromFile(filepath.Join(p.Dir, info.Name()))
						if err != nil {
							return err
						}
						for _, id := range ids {
							if _, has := idsToDestroy[id]; has {
								log.Infof(
									"Not destroying related resource %s, %s references it",
									id,
									info.Name())
								delete(idsToDestroy, id)
							}
						}
					}
					return nil
				})
			if err != nil {
				return err
			}
			// Delete any resources that are no longer referenced
			for id := range idsToDestroy {
				err = p.doDestroy(instance.ID(id), false)
				if err != nil {
					return err
				}
			}
		}
	}
	err := p.fs.Remove(fp)
	if err != nil {
		return err
	}
	return p.terraformApply()
}

// parseAttachTagFromFile parses the file at the given path and returns value of
// the "infrakit.attach" tag
func parseAttachTagFromFile(fp string) ([]string, error) {
	buff, err := ioutil.ReadFile(fp)
	if err != nil {
		log.Warningln("Cannot load file to destroy:", err)
		return nil, err
	}
	tf := TFormat{}
	if err = types.AnyBytes(buff).Decode(&tf); err != nil {
		return nil, err
	}
	vmType, _, vmProps, err := FindVM(&tf)
	if err != nil {
		return nil, err
	}
	tags := parseTerraformTags(vmType, vmProps)
	if attachTag, has := tags[attachTag]; has {
		return strings.Split(attachTag, ","), nil
	}
	return []string{}, nil
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (p *plugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	log.Debugln("describe-instances", tags)
	// Acquire lock since we are reading all files and potentially running "terraform show"
	for {
		if err := p.fsLock.TryLock(); err == nil {
			defer p.fsLock.Unlock()
			break
		}
		time.Sleep(time.Second)
	}

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
	// now we scan for <instance_type.instance-<timestamp>> as keys
	for vmType, vm := range localSpecs {
	scan:
		for vmName, vmProps := range vm {
			// Only process valid instance-xxxx resources
			matches := re.FindStringSubmatch(string(vmName))
			if len(matches) == 3 {
				id := matches[2]

				inst := instance.Description{
					Tags:      parseTerraformTags(vmType, vmProps),
					ID:        instance.ID(id),
					LogicalID: terraformLogicalID(vmProps),
				}

				if properties {
					if vms, has := terraformShowResult[vmType]; has {
						if details, has := vms[vmName]; has {
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

// parseTerraformTags parses the platform-specific tags into a generic map
func parseTerraformTags(vmType TResourceType, m TResourceProperties) map[string]string {
	tags := map[string]string{}
	switch vmType {
	case VMAmazon, VMAzure, VMDigitalOcean, VMGoogleCloud:
		if tagsMap, ok := m["tags"].(map[string]interface{}); ok {
			for k, v := range tagsMap {
				tags[k] = fmt.Sprintf("%v", v)
			}
		} else {
			log.Errorf("parseTerraformTags: invalid %v tags value: %v", vmType, reflect.TypeOf(m["tags"]))
		}
	case VMSoftLayer, VMIBMCloud:
		if tagsSlice, ok := m["tags"].([]interface{}); ok {
			for _, v := range tagsSlice {
				value := fmt.Sprintf("%v", v)
				if strings.Contains(value, ":") {
					log.Debugln("parseTerraformTags system tags detected v=", v)
					// This assumes that the first colon is separating the key and the value of the tag.
					// This is done so that colons are valid characters in the value.
					vv := strings.SplitN(value, ":", 2)
					// Commas are not valid tag characters so a space was used, change back to a common
					// for tag values that are a slice
					if vv[0] == attachTag {
						vv[1] = strings.Replace(vv[1], " ", ",", -1)
					}
					tags[vv[0]] = vv[1]
				} else {
					tags[value] = "" // for list but no ':"
				}
			}
		} else {
			log.Errorln("parseTerraformTags: invalid %v tags value: %v", vmType, reflect.TypeOf(m["tags"]))
		}
	}
	log.Debugln("parseTerraformTags return tags", tags)
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

// importResource imports the resource with the given ID into terraform and creates a
// .tf.json file based on the given spec
func (p *plugin) importResource(resourceID string, spec group.Spec, markBootstrap bool) (*instance.ID, error) {
	// Acquire lock since we are creating a tf.json file and updating terraform state
	var filename string
	for {
		if err := p.fsLock.TryLock(); err == nil {
			defer p.fsLock.Unlock()
			filename = ensureUniqueFile(p.Dir)
			break
		}
		log.Infoln("Can't acquire fsLock on importResource, waiting")
		time.Sleep(time.Second)
	}

	// Get the instance properties we care about
	groupProps, err := group_types.ParseProperties(spec)
	if err != nil {
		return nil, err
	}
	tf := TFormat{}
	err = groupProps.Instance.Properties.Decode(&tf)
	if err != nil {
		return nil, err
	}
	specVMType, _, specVMProps, err := FindVM(&tf)
	if err != nil {
		return nil, err
	}
	if specVMProps == nil {
		return nil, fmt.Errorf("Missing resource properties")
	}

	// Only import if terraform is not already managing
	// TODO(kaufers): Could instead check tag value via metadata plugin
	existingVMs, err := doTerraformShow(p.Dir, specVMType)
	if err != nil {
		return nil, err
	}
	for name, props := range existingVMs {
		if idVal, has := props["id"]; has {
			idStr := fmt.Sprintf("%v", idVal)
			if idStr == resourceID {
				log.Infof("Bootstrap resource %v with ID %v is already managed by terraform", name, idStr)
				id := instance.ID(name)
				return &id, nil
			}
		} else {
			log.Warnf("Resource %v is missing 'id' prop", name)
		}
	}

	// Import into terraform
	log.Infof("Importing %v %v into terraform ...", specVMType, resourceID)
	command := exec.Command(fmt.Sprintf("terraform import %v.%v %s", specVMType, filename, resourceID)).InheritEnvs(true).WithDir(p.Dir)
	if err = command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start(); err != nil {
		p.cleanupFailedImport(specVMType, filename)
		return nil, err
	}
	if err = command.Wait(); err != nil {
		p.cleanupFailedImport(specVMType, filename)
		return nil, err
	}

	// Parse the terraform show output
	importedProps, err := doTerraformShowForInstance(p.Dir, fmt.Sprintf("%v.%v", specVMType, filename))
	if err != nil {
		p.cleanupFailedImport(specVMType, filename)
		return nil, err
	}

	// Merge in the group tags
	tags := map[string]string{
		"infrakit.group": string(spec.ID),
	}
	if markBootstrap {
		tags["infrakit.config_sha"] = "bootstrap"
	}
	mergeTagsIntoVMProps(specVMType, specVMProps, tags)
	// Write out tf.json file
	log.Infoln("Using spec for import", specVMProps)
	if err = p.writeTfJSONForImport(specVMProps, importedProps, specVMType, filename); err != nil {
		p.cleanupFailedImport(specVMType, filename)
		return nil, err
	}
	id := instance.ID(filename)
	return &id, p.terraformApply()
}

// cleanupFailedImport removes the resource from the terraform state file
func (p *plugin) cleanupFailedImport(vmType TResourceType, vmName string) {
	command := exec.Command(fmt.Sprintf("terraform state rm %v.%v", vmType, vmName)).InheritEnvs(true).WithDir(p.Dir)
	err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start()
	if err == nil {
		command.Wait()
	}
}

// writeTfJSONForImport writes the .tf.json file for the imported resource
func (p *plugin) writeTfJSONForImport(specProps, importedProps TResourceProperties, vmType TResourceType, filename string) error {
	// Build the props for the tf.json file
	finalProps := TResourceProperties{}
	for k := range specProps {
		// Ignore certain keys in spec
		if k == PropScope {
			continue
		}
		if k == PropHostnamePrefix {
			k = "hostname"
		}
		v, has := importedProps[k]
		if !has {
			log.Warningf("Imported terraform resource missing '%s' property, not setting", k)
			continue
		}
		finalProps[k] = v
	}
	// Also honor "tags" on imported resource, merge with any in the spec
	mergeProp(importedProps, specProps, "tags")
	if tags, has := specProps["tags"]; has {
		finalProps["tags"] = tags
	}

	// Create the spec and write out the tf.json file
	tf := TFormat{
		Resource: map[TResourceType]map[TResourceName]TResourceProperties{
			vmType: {
				TResourceName(filename): finalProps,
			},
		},
	}
	buff, err := json.MarshalIndent(tf, "  ", "  ")
	log.Debugln("writeTfJSONForImport", filename, "data=", string(buff), "err=", err)
	if err != nil {
		return err
	}
	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, filename+".tf.json"), buff, 0644)
	if err != nil {
		return err
	}
	return nil
}

// mergeProps ensures that the peroperty at the given key in the "source" is set
// on the "dest"; maps and slices are merged, other types are overriden
func mergeProp(source, dest TResourceProperties, key string) {
	sourceData, has := source[key]
	if !has {
		// The key is not on the source, no-op
		return
	}
	destData, has := dest[key]
	if !has {
		// The key is not on the destination, just override with the source
		dest[key] = sourceData
		return
	}

	if sourceDataSlice, ok := sourceData.([]interface{}); ok {
		// Merge slice elements
		if destDataSlice, ok := destData.([]interface{}); ok {
			for _, sourceElement := range sourceDataSlice {
				// "tags" are unique, they are key/value pairs delimited by ":"
				var prefix string
				if key == "tags" {
					if sourceElementStr, ok := sourceElement.(string); ok {
						prefix = strings.Split(sourceElementStr, ":")[0] + ":"
					}
				}
				match := false
				for i, destElement := range destDataSlice {
					if prefix != "" {
						if strings.HasPrefix(destElement.(string), prefix) {
							destDataSlice[i] = sourceElement
							match = true
							break
						}
					} else if sourceElement == destElement {
						match = true
						break
					}
				}
				if !match {
					destDataSlice = append(destDataSlice, sourceElement)
				}
			}
			dest[key] = destDataSlice
		} else {
			log.Errorf(
				"mergeProp: invalid '%v' prop value on spec, expected %v, actual %v",
				key,
				reflect.TypeOf(sourceData),
				reflect.TypeOf(destDataSlice),
			)
			dest[key] = sourceData
		}
	} else if sourceDataMap, ok := sourceData.(map[string]interface{}); ok {
		// Merge map elements
		if destDataMap, ok := destData.(map[string]interface{}); ok {
			for k, v := range sourceDataMap {
				destDataMap[k] = v
			}
		} else {
			log.Errorf(
				"mergeProp: invalid '%v' prop value on spec, expected %v, actual %v",
				key,
				reflect.TypeOf(sourceData),
				reflect.TypeOf(destDataMap),
			)
			dest[key] = sourceData
		}
	} else {
		// Not a complex type, override
		dest[key] = sourceData
	}
}

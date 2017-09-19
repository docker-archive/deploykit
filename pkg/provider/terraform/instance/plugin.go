package instance

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
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/deckarep/golang-set"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
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
	// attachTag contains a space separated list of IDs that are to the instance
	attachTag = "infrakit.attach"

	// scopeDedicated is the scope key for dedicated resources
	scopeDedicated = "dedicated"

	// scopeGlobal is the scope key for global resources
	scopeGlobal = "global"
)

// tfFileRegex is used to determine the all terraform files; files with a ".new" suffix
// have not yet been processed by terraform
var tfFileRegex = regexp.MustCompile("(^.*).tf.json([.new]*)$")

// dedicatedScopedFileRegex is used to determine the scope ID and the instance
// ID (instance-XXXX|logicalID) for a file that contains dedicated resources
var dedicatedScopedFileRegex = regexp.MustCompile("^([a-z]*){1}_dedicated_([a-zA-Z0-9-]*).tf.json([.new]*)$")

// instanceTfFileRegex is used to determine the files that contain a terraform instance
// definition; files with a ".new" suffix have not yet been processed by terraform
var instanceTfFileRegex = regexp.MustCompile("(^instance-[0-9]+)(.tf.json)([.new]*)$")

// instNameRegex is used to determine the name of an instance
var instNameRegex = regexp.MustCompile("(.*)(instance-[0-9]+)")

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
	envs         []string
}

// ImportResource defines a resource that should be imported
type ImportResource struct {
	ResourceID           *string
	ResourceType         *TResourceType
	ResourceName         *TResourceName      // Name of resource in the instance spec
	ResourceProps        TResourceProperties // Populated via tf show
	SpecProps            TResourceProperties // Parsed from instance spec
	FinalProps           TResourceProperties // Properties for the tf.json.new file
	FinalResourceName    TResourceName       // Formatted resource name
	FinalFilename        string              // Filename for the tf.json.new file
	AlreadyImported      bool                // Track if this resource already exists in tf state
	SuccessfullyImported bool                // Track if this resource was imported
}

// ImportOptions defines the resources that should be imported into terraform before
// the plugin is started
type ImportOptions struct {
	InstanceSpec *instance.Spec
	Resources    []*ImportResource
}

// NewTerraformInstancePlugin returns an instance plugin backed by disk files.
func NewTerraformInstancePlugin(dir string, pollInterval time.Duration, standalone bool, envs []string, importOpts *ImportOptions) instance.Plugin {
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
		envs:         envs,
	}
	if err := p.processImport(importOpts); err != nil {
		panic(err)
	}
	// Ensure that tha apply goroutine is always running; it will only run "terraform apply"
	// if the current node is the leader. However, when leadership changes, a Provision is
	// not guaranteed to be executed so we need to create the goroutine now.
	p.terraformApply()
	return &p
}

// processImport imports the resource with the given ID based on the instance Spec;
// after the resource is imported, a tf.json file is also generated so that the
// resource is not orphaned in terraform.
func (p *plugin) processImport(importOpts *ImportOptions) error {
	if importOpts == nil {
		return nil
	}

	if importOpts.InstanceSpec == nil {
		if len(importOpts.Resources) > 0 {
			return fmt.Errorf("Import instance spec required with imported resources")
		}
		// Values are empty, nothing to import
		return nil
	}
	if len(importOpts.Resources) == 0 {
		return fmt.Errorf("Resources required with import instance spec")
	}
	for _, res := range importOpts.Resources {
		if res.ResourceName == nil {
			log.Infof("Processing import for resource: %v:%v",
				string(*res.ResourceType),
				string(*res.ResourceID))
		} else {
			log.Infof("Processing import for resource: %v:%v:%v",
				string(*res.ResourceType),
				string(*res.ResourceName),
				string(*res.ResourceID))
		}
	}

	// Define the functions for import and import the VM
	fns := importFns{
		tfShow: p.doTerraformShow,
		tfImport: func(resType TResourceType, resName, id string) error {
			command := exec.Command(fmt.Sprintf("terraform import %v.%v %s", resType, resName, id)).
				InheritEnvs(true).
				WithEnvs(p.envs...).
				WithDir(p.Dir)
			if err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start(); err != nil {
				return err
			}
			return command.Wait()
		},
		tfShowInst: p.doTerraformShowForInstance,
		tfClean:    p.cleanupFailedImport,
	}
	err := p.importResources(fns, importOpts.Resources, importOpts.InstanceSpec)
	if err != nil {
		log.Errorf("Failed to import instances, error: %v", err)
		return err
	}
	return nil
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

// scanLocalFiles reads the filesystem and loads all tf.json and tf.json.new files
func (p *plugin) scanLocalFiles() (map[TResourceType]map[TResourceName]TResourceProperties, error) {

	vms := map[TResourceType]map[TResourceName]TResourceProperties{}

	fs := &afero.Afero{Fs: p.fs}
	// just scan the directory for the instance-*.tf.json files
	err := fs.Walk(p.Dir,

		func(path string, info os.FileInfo, err error) error {
			matches := instanceTfFileRegex.FindStringSubmatch(info.Name())

			if len(matches) == 4 {
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
		// Encode user data
		if data, has := properties["user_data"]; has {
			properties["user_data"] = base64.StdEncoding.EncodeToString([]byte(data.(string)))
		}
	case VMDigitalOcean:
		// Encode user data
		if data, has := properties["user_data"]; has {
			properties["user_data"] = base64.StdEncoding.EncodeToString([]byte(data.(string)))
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

// renderInstVars applies the "/self/instId", "/self/logicalId", and "/self/dedicated/attachId" variables
// as global options on the input properties
func renderInstVars(props *TResourceProperties, id instance.ID, logicalID *instance.LogicalID, dedicatedAttachKey string) error {
	data, err := json.Marshal(props)
	if err != nil {
		return err
	}
	t, err := template.NewTemplate("str://"+string(data), template.Options{})
	if err != nil {
		return err
	}
	// Instance ID is always supplied
	t = t.Global("/self/instId", id)
	// LogicalID and dedicated attach key values are optional
	if logicalID != nil {
		t = t.Global("/self/logicalId", string(*logicalID))
	}
	if dedicatedAttachKey != "" {
		t = t.Global("/self/dedicated/attachId", dedicatedAttachKey)
	}
	result, err := t.Render(nil)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(result), &props)
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

// decomposedFiles is populated by the decompose function
type decomposedFiles struct {
	FileMap            map[string]*TFormat
	CurrentFiles       map[string]struct{}
	DedicatedAttachKey string
}

// decompose splits the data in the TFormat object into one or more terraform specs, each
// corresponding to a file that should be created. The properties of the VM resource are
// update to use the generated name and the given vmProperties.
func (p *plugin) decompose(logicalID *instance.LogicalID, generatedName string, tf *TFormat, vmType TResourceType, vmProperties TResourceProperties) (*decomposedFiles, error) {
	// Map file names to the data in each file based on the "@scope" property:
	// - @default: resources in same "instance-xxxx" file as VM
	// - @dedicated: resources in different file as VM using the logical ID (<scopeID>_dedicated_<logicalID>) or with
	//   the same generated ID (<scopeID>_dedicated_instance-xxxx)
	// - <other>: resource defined in different file with a scope identifier (<other>_global)
	fileMap := make(map[string]*TFormat)
	// Track the dedicated attach ID for this VM, this is set if there is an orphaned dedicated
	// file that matches the desired format
	dedicatedAttachKey := ""

	// Track current files, used to determine existing dedicated and global resources
	currentFiles := make(map[string]map[TResourceType]map[TResourceName]TResourceProperties)

	for resourceType, resourceObj := range tf.Resource {
		vmList := mapset.NewSetFromSlice(VMTypes)
		for resourceName, resourceProps := range resourceObj {
			var newResourceName string
			if vmList.Contains(resourceType) {
				// Overwrite with the changes to the VM properties
				resourceProps = vmProperties
				newResourceName = generatedName
			} else {
				if logicalID == nil {
					newResourceName = fmt.Sprintf("%s-%s", generatedName, resourceName)
				} else {
					newResourceName = fmt.Sprintf("%s-%s", string(*logicalID), resourceName)
				}
			}
			// Determine the scope value (default to 'default')
			var scope string
			if s, has := resourceProps[PropScope]; has {
				scope = s.(string)
				delete(resourceProps, PropScope)
			} else {
				scope = ValScopeDefault
			}
			// Determine the filename and resource name based off of the scope value
			var filename string
			if scope == ValScopeDefault {
				// Default scope, filename is just the resource name (instance-XXXX)
				filename = generatedName
			} else if strings.HasPrefix(scope, ValScopeDedicated) {
				// Get current files
				if len(currentFiles) == 0 {
					if files, err := p.listCurrentTfFiles(); err == nil {
						currentFiles = files
					} else {
						return nil, err
					}
				}
				// Dedicated scope, filename has a scope identifier and the generated name or logical
				// ID: <scope-id>_dedicated_<instance-XXXX|logicalID>
				var identifier string
				if strings.Contains(scope, "-") {
					identifier = strings.SplitN(scope, "-", 2)[1]
				} else {
					identifier = "default"
				}
				// And the resource name as <scopeID>-<instance-XXXX|logicalID>-<resourceName>
				var key string
				if logicalID == nil {
					// On a rolling update, the dedicated file for a scaler group is not removed, search
					// for an orphaned file with the appropriate format to attach
					allKeys, orphanKeys := findDedicatedAttachmentKeys(currentFiles, identifier)
					if len(orphanKeys) == 0 {
						// No orphans, choose the lowest available index based on the existing files
						index := 1
						for ; ; index = index + 1 {
							match := false
							for _, existingKey := range allKeys {
								if existingKey == strconv.Itoa(index) {
									match = true
									break
								}
							}
							if !match {
								break
							}
						}
						key = strconv.Itoa(index)
						log.Infof("No orphaned attachment with prefix '%v-%s', using current name: %s", identifier, scopeDedicated, key)
					} else {
						// At least 1 orphaned file exists, pick the index with the lowest index
						key = getLowestDedicatedOrphanIndex(orphanKeys)
						for _, instID := range orphanKeys {
							key = instID
							break
						}
						log.Infof("Using orphaned attachment '%s' for prefix '%s-%s'", key, identifier, scopeDedicated)
					}
				} else {
					key = string(*logicalID)
				}
				filename = fmt.Sprintf("%s_%s_%s", identifier, scopeDedicated, key)
				dedicatedAttachKey = key
				newResourceName = fmt.Sprintf("%s-%s-%s", identifier, key, resourceName)
			} else {
				// Get current files
				if len(currentFiles) == 0 {
					if files, err := p.listCurrentTfFiles(); err == nil {
						currentFiles = files
					} else {
						return nil, err
					}
				}
				// Global scope, filename is just the given scope with a "global" suffix
				filename = fmt.Sprintf("%s_%s", scope, scopeGlobal)
				// And the resource name has the given scope as the prefix
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

	currentFilenames := make(map[string]struct{}, len(currentFiles))
	for filename := range currentFiles {
		currentFilenames[filename] = struct{}{}
	}
	result := decomposedFiles{
		FileMap:            fileMap,
		CurrentFiles:       currentFilenames,
		DedicatedAttachKey: dedicatedAttachKey,
	}
	return &result, nil
}

// getLowestDedicatedOrphanIndex gets the lowest numerical slice value
func getLowestDedicatedOrphanIndex(data []string) string {
	// All values should be ints as strings (but handle non-ints), convert and sort
	ints := []int{}
	other := []string{}
	for _, v := range data {
		if intVal, err := strconv.Atoi(v); err == nil {
			ints = append(ints, intVal)
		} else {
			other = append(other, v)
		}
	}
	if len(ints) > 0 {
		sort.Ints(ints)
		return strconv.Itoa(ints[0])
	}
	sort.Strings(other)
	return other[0]
}

// writeTerraformFiles writes *.tf.json[.new] files for each entry in the given fileMap
func (p *plugin) writeTerraformFiles(fileMap map[string]*TFormat, currentFiles map[string]struct{}) ([]string, error) {
	// First verify that there are no formatting errors
	dataMap := make(map[string][]byte, len(fileMap))
	for filename, tfVal := range fileMap {
		buff, err := json.MarshalIndent(tfVal, "  ", "  ")
		if err != nil {
			return []string{}, err
		}
		dataMap[filename] = buff
	}

	// Track files written
	paths := []string{}

	// And write out each file, override data in tf.json if it already exists
	for filename, buff := range dataMap {
		var path string
		if _, has := currentFiles[filename+".tf.json"]; has {
			path = filepath.Join(p.Dir, filename+".tf.json")
			log.Infof("Overriding data in file: %v", path)
		} else {
			path = filepath.Join(p.Dir, filename+".tf.json.new")
		}
		log.Debugln("writeTerraformFiles", path, "data=", string(buff))
		paths = append(paths, path)
		err := afero.WriteFile(p.fs, path, buff, 0644)
		if err != nil {
			return paths, err
		}
	}
	return paths, nil
}

// listCurrentTfFiles populates the map with the names of all tf.json and tf.json.new files
func (p *plugin) listCurrentTfFiles() (map[string]map[TResourceType]map[TResourceName]TResourceProperties, error) {
	// TODO(kaufers): Replace scanLocalFiles with this function once this plugin is updated
	//                to handle any resource type instead of only VMs
	result := make(map[string]map[TResourceType]map[TResourceName]TResourceProperties)
	fs := &afero.Afero{Fs: p.fs}
	err := fs.Walk(p.Dir,
		func(path string, info os.FileInfo, err error) error {
			matches := tfFileRegex.FindStringSubmatch(info.Name())
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
				props := make(map[TResourceType]map[TResourceName]TResourceProperties)
				for resType, resNameProps := range tf.Resource {
					for resName, resProps := range resNameProps {
						if _, has := props[resType]; !has {
							props[resType] = make(map[TResourceName]TResourceProperties)
						}
						props[resType][resName] = resProps
					}
				}
				result[info.Name()] = props
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// findOrphanedDedicatedAttachmentKeys proceeses the current files to determine:
// - All files that match the scope patten (ie, <scopeID>_dedicated_*)
// - A file with the given patten that is not already attached to an instance
// Returns all matching dedicated keys and those that are orphaned
func findDedicatedAttachmentKeys(currentFiles map[string]map[TResourceType]map[TResourceName]TResourceProperties, scopeID string) ([]string, []string) {
	// Find all files that match this scope ID and scope
	allFilesMap := make(map[string]string)
	// And those that are orphaned
	orphanedFilesMap := make(map[string]string)

	for filename := range currentFiles {
		matches := dedicatedScopedFileRegex.FindStringSubmatch(filename)
		if len(matches) != 4 {
			continue
		}
		if matches[1] != scopeID {
			log.Debugf("Ignoring file '%s', scope ID '%s' does not match", filename, scopeID)
			continue
		}
		log.Infof("Found candidate file '%s' for scope ID '%s'", filename, scopeID)
		fileKey := strings.Split(filename, ".")[0]
		allFilesMap[fileKey] = matches[2]
		orphanedFilesMap[fileKey] = matches[2]
	}
	if len(allFilesMap) == 0 {
		log.Infof("No candidate attchment files for scope ID '%s'", scopeID)
		return []string{}, []string{}
	}
	// Prune the candidate files that already have attachments
	supportedVMs := mapset.NewSetFromSlice(VMTypes)
	for filename, resTypeNameProps := range currentFiles {
		matches := instanceTfFileRegex.FindStringSubmatch(filename)
		if len(matches) != 4 {
			continue
		}
		for resType, resNameProps := range resTypeNameProps {
			if !supportedVMs.Contains(resType) {
				continue
			}
			for _, vmProps := range resNameProps {
				tags := parseTerraformTags(resType, vmProps)
				attachTag, has := tags[attachTag]
				if !has {
					continue
				}
				for _, tag := range strings.Split(attachTag, ",") {
					if _, contains := allFilesMap[tag]; contains {
						log.Infof("Attachment '%s' is used in %s for scope ID '%s'", tag, filename, scopeID)
						delete(orphanedFilesMap, tag)
					}
				}
			}
		}
	}
	allMatches := []string{}
	for _, v := range allFilesMap {
		allMatches = append(allMatches, v)
	}
	orphans := []string{}
	for _, v := range orphanedFilesMap {
		orphans = append(orphans, v)
	}
	log.Infof("Detected %v matching files and %v orphan attachments for scope ID '%v': %v",
		len(allMatches),
		len(orphans),
		scopeID,
		orphans)
	return allMatches, orphans
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
	if f, err := os.Open(filepath.Join(dir, n) + ".tf.json.new"); err == nil {
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

	// Decode the given spec and find the VM resource
	tf := TFormat{}
	err := spec.Properties.Decode(&tf)
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
	// Decompose the spec into scope'd files
	decomposedFiles, err := p.decompose(spec.LogicalID, name, &tf, vmType, vmProps)
	if err != nil {
		return nil, err
	}
	// Render any instance specific variables
	if err = renderInstVars(&vmProps, id, spec.LogicalID, decomposedFiles.DedicatedAttachKey); err != nil {
		return nil, err
	}
	// Handle any platform specific updates to the VM properties prior to writing out
	platformSpecificUpdates(vmType, TResourceName(name), spec.LogicalID, vmProps)
	// Write out the tf.json[.new] files
	if _, err = p.writeTerraformFiles(decomposedFiles.FileMap, decomposedFiles.CurrentFiles); err != nil {
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

	tf, filename, err := p.parseFileForInstanceID(instance)
	if err != nil {
		return err
	}

	vmType, vmName, vmProps, err := FindVM(tf)
	if err != nil {
		return err
	}

	if len(vmProps) == 0 || vmName != TResourceName(string(instance)) {
		return fmt.Errorf("not found:%v", instance)
	}

	mergeTagsIntoVMProps(vmType, vmProps, labels)

	buff, err := json.MarshalIndent(tf, "  ", "  ")
	if err != nil {
		return err
	}
	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, filename), buff, 0644)
	if err != nil {
		return err
	}
	return p.terraformApply()
}

// Destroy terminates an existing instance.
func (p *plugin) Destroy(instID instance.ID, context instance.Context) error {
	processAttach := true
	if context == instance.RollingUpdate {
		// Do not destroy related resources since this instance will be re-provisioned
		processAttach = false
	}
	err := p.doDestroy(instID, processAttach, true)
	return err
}

// doDestroy terminates an existing instance and optionally terminates any related
// resources
func (p *plugin) doDestroy(inst instance.ID, processAttach, executeTfApply bool) error {
	// Acquire lock
	for {
		if err := p.fsLock.TryLock(); err == nil {
			defer p.fsLock.Unlock()
			break
		}
		time.Sleep(time.Second)
	}

	tf, filename, err := p.parseFileForInstanceID(inst)
	if err != nil {
		return err
	}

	log.Debugf("Destroying instance %v, processAttach: %v", filename, processAttach)

	// Optionally destroy the related resources
	if processAttach {
		// Get an referenced resources in the "infrakit.attach" tag
		var attachIDs []string
		attachIDs, err = parseAttachTag(tf)
		if err != nil {
			return err
		}
		if len(attachIDs) > 0 {
			idsToDestroy := make(map[string]struct{})
			for _, attachID := range attachIDs {
				idsToDestroy[attachID] = struct{}{}
			}
			// Load all other instance files and determine other references exist
			fs := &afero.Afero{Fs: p.fs}
			err = fs.Walk(p.Dir,
				func(path string, info os.FileInfo, err error) error {
					matches := instanceTfFileRegex.FindStringSubmatch(info.Name())
					// Note that the current instance (being destroyed) still exists; filter
					// this file out.
					if len(matches) == 4 && filename != info.Name() {
						// Load this file
						buff, err := afero.ReadFile(p.fs, filepath.Join(p.Dir, info.Name()))
						if err != nil {
							return err
						}
						tFormat := TFormat{}
						if err = types.AnyBytes(buff).Decode(&tFormat); err != nil {
							return err
						}
						ids, err := parseAttachTag(&tFormat)
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
				err = p.doDestroy(instance.ID(id), false, false)
				if err != nil {
					return err
				}
			}
		}
	}
	err = p.fs.Remove(filepath.Join(p.Dir, filename))
	if err != nil {
		return err
	}
	if executeTfApply {
		return p.terraformApply()
	}
	return nil
}

// parseAttachTag parses the file at the given path and returns value of
// the "infrakit.attach" tag
func parseAttachTag(tf *TFormat) ([]string, error) {
	vmType, _, vmProps, err := FindVM(tf)
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

			if result, err := p.doTerraformShow(p.Dir, []TResourceType{vmResourceType}, nil); err == nil {
				terraformShowResult = result
			} else {
				// Don't blow up... just do best and show what we can find.
				log.Warnln("cannot terraform show:", err)
			}
		}
	}

	result := []instance.Description{}
	// now we scan for <instance_type.instance-<timestamp>> as keys
	for vmType, vm := range localSpecs {
	scan:
		for vmName, vmProps := range vm {
			// Only process valid instance-xxxx resources
			matches := instNameRegex.FindStringSubmatch(string(vmName))
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

// External functions using during import; broken out for testing
type importFns struct {
	tfShow     func(dir string, resTypes []TResourceType, propFilter []string) (map[TResourceType]map[TResourceName]TResourceProperties, error)
	tfImport   func(resType TResourceType, filename, resID string) error
	tfShowInst func(dir, id string) (TResourceProperties, error)
	tfClean    func(resType TResourceType, resName string)
}

// importResource imports the resource with the given ID into terraform and creates a
// .tf.json.new file based on the given spec
func (p *plugin) importResources(fns importFns, resources []*ImportResource, spec *instance.Spec) error {
	// Acquire lock since we are creating a tf.json.new file and updating terraform state
	var vmResName string
	for {
		if err := p.fsLock.TryLock(); err == nil {
			defer p.fsLock.Unlock()
			vmResName = ensureUniqueFile(p.Dir)
			break
		}
		log.Infoln("Can't acquire fsLock on importResource, waiting")
		time.Sleep(time.Second)
	}

	// Parse the instance spec
	tf := TFormat{}
	err := spec.Properties.Decode(&tf)
	if err != nil {
		return err
	}
	specVMType, _, specVMProps, err := FindVM(&tf)
	if err != nil {
		return err
	}
	if specVMProps == nil {
		return fmt.Errorf("Missing resource properties")
	}

	// Map resources to import with resources in the spec
	var specLogicalID instance.LogicalID
	if spec.Tags != nil {
		if logicalID, has := spec.Tags["LogicalID"]; has {
			specLogicalID = instance.LogicalID(logicalID)
		}
	}
	decomposedFiles, err := p.decompose(&specLogicalID, vmResName, &tf, specVMType, specVMProps)
	if err != nil {
		return err
	}
	err = determineImportInfo(resources, decomposedFiles)
	if err != nil {
		return err
	}

	// Check if terraform is already managing the resource(s) being imported
	showResourceTypesMap := map[TResourceType]struct{}{}
	for _, r := range resources {
		showResourceTypesMap[*r.ResourceType] = struct{}{}
	}
	showResourceTypes := []TResourceType{}
	for resType := range showResourceTypesMap {
		showResourceTypes = append(showResourceTypes, resType)
	}
	existingResources, err := fns.tfShow(p.Dir, showResourceTypes, []string{"id"})
	if err != nil {
		return err
	}
	for _, r := range resources {
		if resNameProps, has := existingResources[*r.ResourceType]; has {
			log.Infof("Terraform is managing %v resources of type %v", len(resNameProps), string(*r.ResourceType))
			for name, props := range resNameProps {
				if idVal, has := props["id"]; has {
					idStr := fmt.Sprintf("%v", idVal)
					if idStr == *r.ResourceID {
						log.Infof("Resource %v with ID %v is already managed by terraform", name, idStr)
						r.AlreadyImported = true
						// The filename for the instance needs to be updated with the actual
						// instance timestamp that corresponds to when it was imported
						if r.FinalResourceName == TResourceName(vmResName) {
							r.FinalResourceName = name
							r.FinalFilename = string(name)
							vmResName = string(name)
						}
					} else {
						log.Debugf("Resource with ID '%s' does not match '%s'", idStr, *r.ResourceID)
					}
				} else {
					log.Warnf("Resource %v is missing 'id' prop", name)
				}
			}
		} else {
			log.Infof("Terraform is managing 0 resources of type %v", string(*r.ResourceType))
		}
	}

	// No-op if everything is already imported AND all of the tf.json[.new] files exist
	allImported := true
	for _, r := range resources {
		if !r.AlreadyImported {
			allImported = false
			break
		}
	}
	if allImported {
		// Check for files
		allFilesExist := true
		files, err := ioutil.ReadDir(p.Dir)
		if err != nil {
			return err
		}
		fileMap := make(map[string]struct{})
		for _, f := range files {
			fileMap[f.Name()] = struct{}{}
		}
		for _, r := range resources {
			match := false
			for _, suffix := range []string{".tf.json", ".tf.json.new"} {
				filename := r.FinalFilename + suffix
				if _, has := fileMap[filename]; has {
					log.Infof("File exists for imported resource: %v", filename)
					match = true
					break
				}
			}
			if !match {
				log.Infof("tf.json file with prefix '%v' does not exist for imported resource: %v", r.FinalFilename)
				allFilesExist = false
				break
			}
		}
		if allFilesExist {
			return nil
		}
	}

	// Track any error that would require cleaning the tf state
	var errorToThrow error

	// Import into terraform
	for _, r := range resources {
		if r.AlreadyImported {
			continue
		}
		log.Infof("Importing %v %v into terraform as resource %v ...", string(*r.ResourceType), string(*r.ResourceID), string(r.FinalResourceName))
		r.SuccessfullyImported = true
		if err = fns.tfImport(*r.ResourceType, string(r.FinalResourceName), *r.ResourceID); err != nil {
			errorToThrow = err
			break
		}
	}

	// Parse the terraform show output
	if errorToThrow == nil {
		for _, r := range resources {
			importedProps, err := fns.tfShowInst(p.Dir, fmt.Sprintf("%v.%v", string(*r.ResourceType), r.FinalResourceName))
			if err != nil {
				errorToThrow = err
				break
			}
			r.ResourceProps = importedProps
		}
	}

	// Merge tags from the spec into the imported instance resource tags
	if errorToThrow == nil && spec.Tags != nil && len(spec.Tags) > 0 {
		for _, r := range resources {
			if r.FinalFilename == vmResName {
				// Tags explicitly set on the spec
				mergeTagsIntoVMProps(*r.ResourceType, r.ResourceProps, spec.Tags)
				// "tags" property in the spec instance defn
				mergeProp(r.SpecProps, r.ResourceProps, "tags")
				break
			}
		}
	}

	// Determine the actual property values for the tf.json.new file
	if errorToThrow == nil {
		for _, r := range resources {
			determineFinalPropsForImport(r)
		}
	}

	// Write out the tf.json.new file(s)
	paths := []string{}
	if errorToThrow == nil {
		paths, err = p.writeTfJSONFilesForImport(resources)
		if err != nil {
			errorToThrow = err
		}
	}

	// If there is an err then remove any new state from terraform and any new files
	if errorToThrow != nil {
		for _, r := range resources {
			if r.SuccessfullyImported {
				fns.tfClean(*r.ResourceType, string(r.FinalResourceName))
			}
		}
		for _, path := range paths {
			if err = p.fs.Remove(path); err == nil {
				log.Info("Successfully removed file created by import: %v", path)
			} else {
				log.Info("Failed to remove file %v created by import: %v", path, err)
			}
		}
		return errorToThrow
	}

	return p.terraformApply()
}

// cleanupFailedImport removes the resource from the terraform state file
func (p *plugin) cleanupFailedImport(vmType TResourceType, vmName string) {
	command := exec.Command(fmt.Sprintf("terraform state rm %v.%v", vmType, vmName)).
		InheritEnvs(true).
		WithEnvs(p.envs...).
		WithDir(p.Dir)
	err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start()
	if err == nil {
		command.Wait()
	}
}

// writeTfJSONFilesForImport writes out the final tf.json[.new] file by grouping all
// imported resources by common filename.
func (p *plugin) writeTfJSONFilesForImport(resources []*ImportResource) ([]string, error) {
	// Group resources by common filename
	fileMap := map[string]*TFormat{}
	for _, r := range resources {
		var tf TFormat
		if tfVal, has := fileMap[r.FinalFilename]; has {
			tf = *tfVal
		} else {
			tf = TFormat{
				Resource: map[TResourceType]map[TResourceName]TResourceProperties{},
			}
			fileMap[r.FinalFilename] = &tf
		}
		var resNameProps map[TResourceName]TResourceProperties
		if resNamePropsVal, has := tf.Resource[*r.ResourceType]; has {
			resNameProps = resNamePropsVal
		} else {
			resNameProps = map[TResourceName]TResourceProperties{}
			tf.Resource[*r.ResourceType] = resNameProps
		}
		resNameProps[r.FinalResourceName] = r.FinalProps
	}
	// Retrieve the current files, used so that we do not create a tf.json.new file
	// if a corresponding tf.json file already exists
	files, err := ioutil.ReadDir(p.Dir)
	if err != nil {
		return []string{}, err
	}
	currentFiles := map[string]struct{}{}
	for _, file := range files {
		currentFiles[file.Name()] = struct{}{}
	}
	return p.writeTerraformFiles(fileMap, currentFiles)
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

// parseFileForInstanceID attempts to load a tf.json (or tf.json.new) file, returning
// the parsed spec and the filename parsed
func (p *plugin) parseFileForInstanceID(instance instance.ID) (*TFormat, string, error) {
	// Instance may not have been processed by terraform; attempt to open both files
	filenames := []string{string(instance) + ".tf.json", string(instance) + ".tf.json.new"}
	var filename string
	var buff []byte
	var err error
	for _, filename = range filenames {
		buff, err = afero.ReadFile(p.fs, filepath.Join(p.Dir, filename))
		// Successfully read a file
		if err == nil {
			break
		}
	}
	// Failed to read a file
	if err != nil {
		return nil, "", err
	}
	tf := TFormat{}
	if err = types.AnyBytes(buff).Decode(&tf); err != nil {
		return nil, "", err
	}
	return &tf, filename, nil
}

// determineImportInfo maps the resources being imported to a the specific file and
// resource name based on the instance spec decomposition.
func determineImportInfo(resources []*ImportResource, files *decomposedFiles) error {
	// Resources without a specific name, should match to the single resource in the spec
	noNames := map[TResourceType]*ImportResource{}
	for _, r := range resources {
		if r.ResourceName != nil {
			continue
		}
		resType := *r.ResourceType
		if _, has := noNames[resType]; has {
			return fmt.Errorf("Error importing resources, more then a single non-named resource of type %v", resType)
		}
		noNames[resType] = r
	}
	// Resources with name
	withNames := map[TResourceType]map[TResourceName]*ImportResource{}
	for _, r := range resources {
		if r.ResourceName == nil {
			continue
		}
		resType := *r.ResourceType
		resName := *r.ResourceName
		var nameMap map[TResourceName]*ImportResource
		if existingMap, has := withNames[resType]; has {
			if _, has := existingMap[resName]; has {
				return fmt.Errorf("Error importing resources, duplicate %s resource with name %v", resType, resName)
			}
			nameMap = existingMap
		} else {
			nameMap = make(map[TResourceName]*ImportResource)
		}
		nameMap[resName] = r
		withNames[resType] = nameMap
	}

	for filename, decomposedTf := range files.FileMap {
		for resType, resNameProps := range decomposedTf.Resource {
			for resName, resProps := range resNameProps {
				// Check for instances with matching name
				if nameMap, has := withNames[resType]; has {
					match := false
					for name, r := range nameMap {
						if strings.HasSuffix(string(resName), string(name)) {
							if err := setFinalResourceAndFilename(r, filename, &resName, resProps); err != nil {
								return err
							}
							match = true
							break
						}
					}
					if match {
						continue
					}
				}
				// Check for resource type only
				if r, has := noNames[resType]; has {
					if err := setFinalResourceAndFilename(r, filename, &resName, resProps); err != nil {
						return err
					}
				}
			}
		}
	}
	// Verify that each imported resource matched something in the spec
	for _, r := range resources {
		if r.FinalFilename == "" || r.FinalResourceName == "" {
			if r.ResourceName == nil {
				return fmt.Errorf("Unable to determine import resource in spec for: %v", string(*r.ResourceType))
			}
			return fmt.Errorf(
				"Unable to determine import resource in spec for %v:%v",
				string(*r.ResourceType),
				string(*r.ResourceName),
			)
		}
	}
	return nil
}

// setFinalResourceAndFilename updates the ImportResource struct to set the final
// resource and filename. An error is returned if either of these values have
// already been set.
func setFinalResourceAndFilename(resource *ImportResource, filename string, resourceName *TResourceName, resourceProps TResourceProperties) error {
	if resource.FinalFilename != "" || resource.FinalResourceName != "" {
		if resource.ResourceName == nil {
			return fmt.Errorf(
				"Ambiguous import resource definition %v:%v",
				string(*resource.ResourceType),
				string(*resource.ResourceID),
			)
		}
		return fmt.Errorf(
			"Ambiguous import resource definition %v:%v:%v",
			string(*resource.ResourceType),
			string(*resource.ResourceName),
			string(*resource.ResourceID),
		)
	}
	resource.FinalResourceName = TResourceName(*resourceName)
	resource.FinalFilename = filename
	resource.SpecProps = resourceProps
	return nil
}

// determineFinalPropsForImport set the FinalProps struct value to the properties that
// should be contained in the tf.json file for an imported resource. The property keys
// are those from the spec and the values are current values (from tf show).
func determineFinalPropsForImport(res *ImportResource) {
	log.Infof("Using spec for %v import: %v", string(*res.ResourceType), res.SpecProps)
	finalProps := TResourceProperties{}
	for k := range res.SpecProps {
		// Ignore certain keys in spec
		if k == PropScope {
			continue
		}
		if k == PropHostnamePrefix {
			k = "hostname"
		}
		v, has := res.ResourceProps[k]
		if !has {
			log.Warningf("Imported terraform resource missing '%s' property, not setting", k)
			continue
		}
		finalProps[k] = v
	}
	// Always keep the tags, even if the spec does not have them as a property
	if _, has := finalProps["tags"]; !has {
		if tags, has := res.ResourceProps["tags"]; has {
			finalProps["tags"] = tags
		}
	}
	res.FinalProps = finalProps
}

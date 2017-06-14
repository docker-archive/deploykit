package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// getPlugin returns the terraform instance plugin to use for testing and the
// directory where the .tf.json files should be stored
func getPlugin(t *testing.T) (instance.Plugin, string) {
	dir, err := ioutil.TempDir("", "infrakit-instance-terraform")
	require.NoError(t, err)
	terraform := NewTerraformInstancePlugin(dir)
	terraform.(*plugin).pretend = true
	return terraform, dir
}

func TestHandleProvisionTagsEmptyTagsLogicalID(t *testing.T) {
	logicalID := instance.LogicalID("logical-id-1")
	// Spec with logical ID
	spec := instance.Spec{
		Properties:  nil,
		Tags:        map[string]string{},
		Init:        "",
		Attachments: []instance.Attachment{},
		LogicalID:   &logicalID,
	}
	for _, vmType := range VMTypes {
		props := TResourceProperties{}
		handleProvisionTags(spec, instance.ID("instance-1234"), vmType.(TResourceType), props)
		tags := props["tags"]
		var expectedTags interface{}
		if vmType == VMSoftLayer {
			sort.Strings(props["tags"].([]string))
			// Note that tags are all lowercase
			expectedTags = []string{
				"logicalid:logical-id-1",
				"name:instance-1234"}
		} else {
			expectedTags = map[string]string{
				"LogicalID": "logical-id-1",
				"Name":      "instance-1234"}
		}
		require.Equal(t, expectedTags, tags)
	}
}

func TestHandleProvisionTagsEmptyTagsNoLogicalID(t *testing.T) {
	// Spec without logical ID
	spec := instance.Spec{
		Properties:  nil,
		Tags:        map[string]string{},
		Init:        "",
		Attachments: []instance.Attachment{},
		LogicalID:   nil,
	}
	for _, vmType := range VMTypes {
		props := TResourceProperties{}
		handleProvisionTags(spec, instance.ID("instance-1234"), vmType.(TResourceType), props)
		tags := props["tags"]
		var expectedTags interface{}
		if vmType == VMSoftLayer {
			expectedTags = []string{"name:instance-1234"}
		} else {
			expectedTags = map[string]string{"Name": "instance-1234"}
		}
		require.Equal(t, expectedTags, tags)
	}
}

func TestHandleProvisionTagsWithTagsLogicalID(t *testing.T) {
	logicalID := instance.LogicalID("logical-id-1")
	// Spec with logical ID
	spec := instance.Spec{
		Properties: nil,
		Tags: map[string]string{
			"name": "existing-name",
			"foo":  "bar"},
		Init:        "",
		Attachments: []instance.Attachment{},
		LogicalID:   &logicalID,
	}
	for _, vmType := range VMTypes {
		props := TResourceProperties{}
		handleProvisionTags(spec, instance.ID("instance-1234"), vmType.(TResourceType), props)
		tags := props["tags"]
		var expectedTags interface{}
		if vmType == VMSoftLayer {
			sort.Strings(props["tags"].([]string))
			// Note that tags are all lowercase
			expectedTags = []string{
				"foo:bar",
				"logicalid:logical-id-1",
				"name:existing-name"}
		} else {
			expectedTags = map[string]string{
				"LogicalID": "logical-id-1",
				"name":      "existing-name",
				"foo":       "bar"}
		}
		require.Equal(t, expectedTags, tags)
	}
}

func TestHandleProvisionTagsWithTagsNoLogicalID(t *testing.T) {
	// Spec without logical ID
	spec := instance.Spec{
		Properties: nil,
		Tags: map[string]string{
			"Name": "existing-name",
			"foo":  "bar"},
		Init:        "",
		Attachments: []instance.Attachment{},
		LogicalID:   nil,
	}
	for _, vmType := range VMTypes {
		props := TResourceProperties{}
		handleProvisionTags(spec, instance.ID("instance-1234"), vmType.(TResourceType), props)
		tags := props["tags"]
		var expectedTags interface{}
		if vmType == VMSoftLayer {
			sort.Strings(props["tags"].([]string))
			expectedTags = []string{"foo:bar", "name:existing-name"}
		} else {
			expectedTags = map[string]string{"Name": "existing-name", "foo": "bar"}
		}
		require.Equal(t, expectedTags, tags)
	}
}

func TestMergeInitScriptNoUserDefined(t *testing.T) {
	for _, vmType := range VMTypes {
		initData := "pwd\nls"
		spec := instance.Spec{
			Properties:  nil,
			Tags:        map[string]string{},
			Init:        initData,
			Attachments: []instance.Attachment{},
			LogicalID:   nil,
		}
		// Input properites do not have init data
		props := TResourceProperties{}
		mergeInitScript(spec, instance.ID("instance-1234"), vmType.(TResourceType), props)
		switch vmType {
		case VMAmazon, VMDigitalOcean:
			require.Equal(t,
				TResourceProperties{"user_data": base64.StdEncoding.EncodeToString([]byte(initData))},
				props)
		case VMSoftLayer:
			require.Equal(t,
				TResourceProperties{"user_metadata": initData},
				props)
		case VMAzure:
			require.Equal(t,
				TResourceProperties{"os_profile": map[string]interface{}{"custom_data": initData}},
				props)
		case VMGoogleCloud:
			require.Equal(t,
				TResourceProperties{"metadata_startup_script": initData},
				props)
		default:
			require.Fail(t, fmt.Sprintf("Init script not handled for type: %v", initData))
		}
	}
}

func TestMergeInitScriptWithUserDefined(t *testing.T) {
	for _, vmType := range VMTypes {
		initData := "pwd\nls"
		spec := instance.Spec{
			Properties:  nil,
			Tags:        map[string]string{},
			Init:        initData,
			Attachments: []instance.Attachment{},
			LogicalID:   nil,
		}
		instanceUserData := "set\nifconfig"
		expectedInit := fmt.Sprintf("%s\n%s", instanceUserData, initData)

		// Configure the input properties with init data
		props := TResourceProperties{}
		switch vmType {
		case VMAmazon, VMDigitalOcean:
			props["user_data"] = instanceUserData
		case VMSoftLayer:
			props["user_metadata"] = instanceUserData
		case VMAzure:
			props["os_profile"] = map[string]interface{}{"custom_data": instanceUserData}
		case VMGoogleCloud:
			props["metadata_startup_script"] = instanceUserData
		default:
			require.Fail(t, fmt.Sprintf("Init script not handled for type: %v", vmType))
		}
		// Merge the spec init data with the input properties
		mergeInitScript(spec, instance.ID("instance-1234"), vmType.(TResourceType), props)
		switch vmType {
		case VMAmazon, VMDigitalOcean:
			require.Equal(t,
				TResourceProperties{"user_data": base64.StdEncoding.EncodeToString([]byte(expectedInit))},
				props)
		case VMSoftLayer:
			require.Equal(t,
				TResourceProperties{"user_metadata": expectedInit},
				props)
		case VMAzure:
			require.Equal(t,
				TResourceProperties{"os_profile": map[string]interface{}{"custom_data": expectedInit}},
				props)
		case VMGoogleCloud:
			require.Equal(t,
				TResourceProperties{"metadata_startup_script": expectedInit},
				props)
		default:
			require.Fail(t, fmt.Sprintf("Init script not handled for type: %v", vmType))
		}
	}
}

func TestProvisionNoResources(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	spec := instance.Spec{
		Properties:  types.AnyString("{}"),
		Tags:        map[string]string{},
		Init:        "",
		Attachments: []instance.Attachment{},
		LogicalID:   nil,
	}
	_, err := terraform.Provision(spec)
	require.Error(t, err)
	require.Equal(t, "no resource section", err.Error())
}

func TestProvisionNoVM(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	spec := instance.Spec{
		Properties:  types.AnyString("{\"resource\": {}}"),
		Tags:        map[string]string{},
		Init:        "",
		Attachments: []instance.Attachment{},
		LogicalID:   nil,
	}
	_, err := terraform.Provision(spec)
	require.Error(t, err)
	require.Equal(t, "not found", err.Error())
}

func TestProvisionNoVMProperties(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	spec := instance.Spec{
		Properties:  types.AnyString("{\"resource\": {\"aws_instance\": {}}}"),
		Tags:        map[string]string{},
		Init:        "",
		Attachments: []instance.Attachment{},
		LogicalID:   nil,
	}
	_, err := terraform.Provision(spec)
	require.Error(t, err)
	require.Equal(t, "no-vm-instance-in-spec", err.Error())
}

func TestProvisionInvalidTemplateProperties(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	spec := instance.Spec{
		Properties:  types.AnyString("{{}"),
		Tags:        map[string]string{},
		Init:        "",
		Attachments: []instance.Attachment{},
		LogicalID:   nil,
	}
	_, err := terraform.Provision(spec)
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "template:"))
}

func TestProvisionInvalidTemplateInit(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	spec := instance.Spec{
		Properties:  types.AnyString("{}"),
		Tags:        map[string]string{},
		Init:        "{{}",
		Attachments: []instance.Attachment{},
		LogicalID:   nil,
	}
	_, err := terraform.Provision(spec)
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "template:"))
}

func TestRunValidateProvisionDescribe(t *testing.T) {
	// Test a softlayer_virtual_guest with an @hostname_prefix
	runValidateProvisionDescribe(t, "softlayer_virtual_guest", `
{
	"resource" : {
		"softlayer_virtual_guest": {
			"host" : {
				"@hostname_prefix": "softlayer-hostname",
				"cores": 2,
				"memory": 2048,
				"tags": [
					"terraform_demo_swarm_mgr_sl"
				],
				"connection": {
					"user": "root",
					"private_key": "${file(\"~/.ssh/id_rsa_de\")}"
				},
				"hourly_billing": true,
				"local_disk": true,
				"network_speed": 100,
				"datacenter": "dal10",
				"os_reference_code": "UBUNTU_14_64",
				"domain": "softlayer.com",
				"ssh_key_ids": [
					"${data.softlayer_ssh_key.public_key.id}"
				],
				"user_metadata": "echo {{ var `+"`/self/instId`"+` }}"
			}
		}
	}
}
`)

	// Test a softlayer_virtual_guest without an @hostname_prefix
	runValidateProvisionDescribe(t, "softlayer_virtual_guest", `
{
	"resource" : {
		"softlayer_file_storage": {
			"worker_file_storage": {
				"iops" : 0.25,
				"type" : "Endurance",
				"datacenter" : "dal10",
				"capacity" : 20
			}
		},
		"softlayer_block_storage": {
			"worker_block_storage": {
				"iops" : 0.25,
				"type" : "Endurance",
				"datacenter" : "dal10",
				"capacity" : 20,
				"os_format_type" : "Linux"
			}
		},
		"softlayer_virtual_guest" : {
			"host": {
				"cores": 2,
				"memory": 2048,
				"tags": [ "terraform_demo_swarm_mgr_sl" ],
				"connection": {
					"user": "root",
					"private_key": "${file(\"~/.ssh/id_rsa_de\")}"
				},
				"hourly_billing": true,
				"local_disk": true,
				"network_speed": 100,
				"datacenter": "dal10",
				"os_reference_code": "UBUNTU_14_64",
				"domain": "softlayer.com",
				"ssh_key_ids": [ "${data.softlayer_ssh_key.public_key.id}" ],
				"user_metadata": "echo {{ var `+"`/self/instId`"+` }}"
			}
		}
	}
}
`)

	// Test a softlayer_virtual_guest with an empty @hostname_prefix
	runValidateProvisionDescribe(t, "softlayer_virtual_guest", `
{
	"resource" : {
		"softlayer_virtual_guest" : {
			"host" : {
				"@hostname_prefix": "   ",
				"cores": 2,
				"memory": 2048,
				"tags": [
					"terraform_demo_swarm_mgr_sl"
				],
				"connection": {
					"user": "root",
					"private_key": "${file(\"~/.ssh/id_rsa_de\")}"
				},
				"hourly_billing": true,
				"local_disk": true,
				"network_speed": 100,
				"datacenter": "dal10",
				"os_reference_code": "UBUNTU_14_64",
				"domain": "softlayer.com",
				"ssh_key_ids": [
					"${data.softlayer_ssh_key.public_key.id}"
				],
				"user_metadata": "echo {{ var `+"`/self/instId`"+` }}"
			}
		}
	}
}
`)

	runValidateProvisionDescribe(t, "aws_instance", `
{
	"resource" : {
		"aws_instance" : {
			"host" : {
				"ami" : "${lookup(var.aws_amis, var.aws_region)}",
				"instance_type" : "m1.small",
				"key_name": "PUBKEY",
				"vpc_security_group_ids" : ["${aws_security_group.default.id}"],
				"subnet_id": "${aws_subnet.default.id}",
				"private_ip": "INSTANCE_LOGICAL_ID",
				"tags" :  {
					"Name" : "web4",
					"InstancePlugin" : "terraform"
				},
				"connection" : {
					"user" : "ubuntu"
				},
				"user_data": "echo {{ var `+"`/self/instId`"+` }}",
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
`)
}

// firstInMap returns the first key/value pair in the given map
func firstInMap(m map[string]interface{}) (string, interface{}) {
	for k, v := range m {
		return k, v
	}
	return "", nil
}

// runValidateProvisionDescribe validates, provisions, and describes instances
// based on the given resource type and properties
func runValidateProvisionDescribe(t *testing.T, resourceType, properties string) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	config := types.AnyString(properties)
	err := terraform.Validate(config)
	require.NoError(t, err)

	// Instance with tags that will not be updated
	logicalID1 := instance.LogicalID("logical.id-1")
	instanceSpec1 := instance.Spec{
		Properties: config,
		Tags: map[string]string{
			"label1": "value1",
			"label2": "value2",
			"LABEL3": "VALUE3",
		},
		Init:        "",
		Attachments: []instance.Attachment{},
		LogicalID:   &logicalID1,
	}
	id1, err := terraform.Provision(instanceSpec1)
	require.NoError(t, err)
	tfPath1 := filepath.Join(dir, string(*id1)+".tf.json")
	_, err = ioutil.ReadFile(tfPath1)
	require.NoError(t, err)

	// Instance with tags that will be updated
	logicalID2 := instance.LogicalID("logical:id-2")
	instanceSpec2 := instance.Spec{
		Properties: config,
		Tags: map[string]string{
			"label1": "value1",
			"label2": "value2",
		},
		Init: "apt-get update -y\n\napt-get install -y software",
		Attachments: []instance.Attachment{
			{
				ID:   "ebs1",
				Type: "ebs",
			},
		},
		LogicalID: &logicalID2,
	}
	id2, err := terraform.Provision(instanceSpec2)
	require.NoError(t, err)
	require.NotEqual(t, id1, id2)

	tfPath2 := filepath.Join(dir, string(*id2)+".tf.json")
	buff, err := ioutil.ReadFile(tfPath2)
	require.NoError(t, err)

	any := types.AnyBytes(buff)
	tformat := TFormat{}
	err = any.Decode(&tformat)
	require.NoError(t, err)

	vmType, _, props, err := FindVM(&tformat)
	require.NoError(t, err)
	require.NotNil(t, props)

	// Unmarshal json for easy access
	var testingData interface{}
	json.Unmarshal([]byte(properties), &testingData)
	m := testingData.(map[string]interface{})

	// More than one resource may be defined.  Loop through them.
	for key, resources := range m["resource"].(map[string]interface{}) {
		resourceName, resource := firstInMap(resources.(map[string]interface{}))
		value, _ := resource.(map[string]interface{})

		// Userdata should have the resource defined data (ie, echo <instId>) with
		// the spec init data appended
		expectedUserData2 := "echo " + string(*id2) + "\n" + instanceSpec2.Init

		switch TResourceType(key) {
		case VMSoftLayer:
			require.Equal(t, conv([]interface{}{
				"terraform_demo_swarm_mgr_sl",
				"label1:value1",
				"label2:value2",
				"name:" + string(*id2),
				"logicalid:logical:id-2",
			}), conv(props["tags"].([]interface{})))
			require.Equal(t, expectedUserData2, props["user_metadata"])

			// If a hostname was specified, the expectation is that the hostname is appended with the logical ID
			if value["@hostname_prefix"] != nil && strings.Trim(value["@hostname_prefix"].(string), " ") != "" {
				expectedHostname := "softlayer-hostname-logical:id-2"
				require.Equal(t, expectedHostname, props["hostname"])
			} else {
				// If no hostname was specified, the hostname should equal the logical ID
				require.Equal(t, "logical:id-2", props["hostname"])
			}
			// Verify the hostname prefix key/value is no longer in the props
			require.Nil(t, props["@hostname_prefix"])

		case VMAmazon:
			require.Equal(t, map[string]interface{}{
				"InstancePlugin": "terraform",
				"label1":         "value1",
				"label2":         "value2",
				"Name":           string(*id2),
				"LogicalID":      "logical:id-2",
			}, props["tags"])
			require.Equal(t, base64.StdEncoding.EncodeToString([]byte(expectedUserData2)), props["user_data"])

		default:
			// Find the resource and make sure the name was updated
			var resourceFound bool
			var name string
			for resourceType, objs := range tformat.Resource {
				if resourceType == TResourceType(key) {
					resourceFound = true
					for k := range objs {
						name = string(k)
						break
					}
				}
			}
			require.True(t, resourceFound)
			// Other resources should be renamed to include the id
			require.Equal(t, name, fmt.Sprintf("%s-%s", string(*id2), resourceName))

		}
	}

	// Expected instances returned from Describe
	var inst1 instance.Description
	var inst2 instance.Description
	switch vmType {
	case VMSoftLayer:
		inst1 = instance.Description{
			ID: *id1,
			Tags: map[string]string{
				"terraform_demo_swarm_mgr_sl": "",
				"label1":                      "value1",
				"label2":                      "value2",
				"label3":                      "value3",
				"name":                        string(*id1),
				"logicalid":                   "logical.id-1",
			},
			LogicalID: &logicalID1,
		}
		inst2 = instance.Description{
			ID: *id2,
			Tags: map[string]string{
				"terraform_demo_swarm_mgr_sl": "",
				"label1":                      "value1",
				"label2":                      "value2",
				"name":                        string(*id2),
				"logicalid":                   "logical:id-2",
			},
			LogicalID: &logicalID2,
		}
	case VMAmazon:
		inst1 = instance.Description{
			ID: *id1,
			Tags: map[string]string{
				"InstancePlugin": "terraform",
				"label1":         "value1",
				"label2":         "value2",
				"LABEL3":         "VALUE3",
				"Name":           string(*id1),
				"LogicalID":      "logical.id-1",
			},
			LogicalID: &logicalID1,
		}
		inst2 = instance.Description{
			ID: *id2,
			Tags: map[string]string{
				"InstancePlugin": "terraform",
				"label1":         "value1",
				"label2":         "value2",
				"Name":           string(*id2),
				"LogicalID":      "logical:id-2",
			},
			LogicalID: &logicalID2,
		}
	}

	// Both instances match: label=value1
	list, err := terraform.DescribeInstances(map[string]string{"label1": "value1"}, false)
	require.NoError(t, err)
	require.Contains(t, list, inst1)
	require.Contains(t, list, inst2)

	// re-label instance2
	err = terraform.Label(*id2, map[string]string{
		"label1": "changed1",
		"label3": "value3",
	})
	require.NoError(t, err)

	buff, err = ioutil.ReadFile(tfPath2)
	require.NoError(t, err)

	any = types.AnyBytes(buff)

	parsed := TFormat{}
	err = any.Decode(&parsed)
	require.NoError(t, err)

	vmType, _, props, err = FindVM(&parsed)
	require.NoError(t, err)
	switch vmType {
	case VMSoftLayer:
		require.Equal(t, conv([]interface{}{
			"terraform_demo_swarm_mgr_sl",
			"label1:changed1",
			"label2:value2",
			"label3:value3",
			"name:" + string(*id2),
			"logicalid:logical:id-2",
		}), conv(props["tags"].([]interface{})))
	case VMAmazon:
		require.Equal(t, map[string]interface{}{
			"InstancePlugin": "terraform",
			"label1":         "changed1",
			"label2":         "value2",
			"label3":         "value3",
			"Name":           string(*id2),
			"LogicalID":      "logical:id-2",
		}, props["tags"])
	}

	// Update expected tags on inst2
	inst2.Tags["label1"] = "changed1"
	inst2.Tags["label3"] = "value3"

	// Only a single match: label1=changed1
	list, err = terraform.DescribeInstances(map[string]string{"label1": "changed1"}, false)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{inst2}, list)

	// Only a single match: label1=value1
	list, err = terraform.DescribeInstances(map[string]string{"label1": "value1"}, false)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{inst1}, list)

	// No matches: label1=foo
	list, err = terraform.DescribeInstances(map[string]string{"label1": "foo"}, false)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{}, list)

	// Destroy, then none should match and 1 file should be removed
	err = terraform.Destroy(*id2, instance.Termination)
	require.NoError(t, err)
	files, err := ioutil.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, filepath.Base(tfPath1), files[0].Name())

	list, err = terraform.DescribeInstances(map[string]string{"label1": "changed1"}, false)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{}, list)

	err = terraform.Destroy(*id1, instance.Termination)
	require.NoError(t, err)
	files, err = ioutil.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 0)
}

func conv(a []interface{}) []string {
	sa := make([]string, len(a))
	for i, x := range a {
		sa[i] = x.(string)
	}
	sort.Strings(sa)
	return sa
}

func TestFindVMNoResource(t *testing.T) {
	tformat := TFormat{}
	_, _, _, err := FindVM(&tformat)
	require.Error(t, err)
	require.Equal(t, "no resource section", err.Error())
}

func TestFindVMEmptyResource(t *testing.T) {
	m := make(map[TResourceType]map[TResourceName]TResourceProperties)
	tformat := TFormat{Resource: m}
	_, _, _, err := FindVM(&tformat)
	require.Error(t, err)
	require.Equal(t, "not found", err.Error())
}

func TestFindVM(t *testing.T) {
	typeMap := make(map[TResourceType]map[TResourceName]TResourceProperties)
	nameMap := make(map[TResourceName]TResourceProperties)
	nameMap["some-name"] = TResourceProperties{"foo": "bar"}
	typeMap[VMSoftLayer] = nameMap
	tformat := TFormat{Resource: typeMap}
	vmType, vmName, props, err := FindVM(&tformat)
	require.NoError(t, err)
	require.Equal(t, VMSoftLayer, vmType)
	require.Equal(t, TResourceName("some-name"), vmName)
	require.Equal(t, TResourceProperties{"foo": "bar"}, props)
}

func TestFirstEmpty(t *testing.T) {
	vms := make(map[TResourceName]TResourceProperties)
	name, props := first(vms)
	require.Equal(t, TResourceName(""), name)
	require.Nil(t, props)
}

func TestFirst(t *testing.T) {
	vms := make(map[TResourceName]TResourceProperties)
	vms["first-name"] = TResourceProperties{"k1": "v1", "k2": "v2"}
	name, props := first(vms)
	require.Equal(t, TResourceName("first-name"), name)
	require.Equal(t, TResourceProperties{"k1": "v1", "k2": "v2"}, props)
}

func TestValidateInvalidJSON(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	config := types.AnyString("not-going-to-decode")
	err := terraform.Validate(config)
	require.Error(t, err)
}

func TestValidate(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	// Should fail with 2 VMs
	props := map[string]map[TResourceType]TResourceProperties{
		"resource": {
			VMSoftLayer: TResourceProperties{},
			VMAmazon:    TResourceProperties{},
		},
	}
	config, err := json.Marshal(props)
	require.NoError(t, err)
	err = terraform.Validate(types.AnyBytes(config))
	require.Error(t, err)
	require.True(t, strings.HasPrefix(
		err.Error(),
		"zero or 1 vm instance per request:"),
		fmt.Sprintf("Error does not have correct prefix: %v", err.Error()),
	)
	// And pass with 1
	delete(props["resource"], VMAmazon)
	require.Equal(t, 1, len(props["resource"]))
	config, err = json.Marshal(props)
	require.NoError(t, err)
	err = terraform.Validate(types.AnyBytes(config))
	require.NoError(t, err)
	// And pass with 0
	delete(props["resource"], VMSoftLayer)
	require.Empty(t, props["resource"])
	config, err = json.Marshal(props)
	require.NoError(t, err)
	err = terraform.Validate(types.AnyBytes(config))
	require.NoError(t, err)
}

func TestAddUserDataNoMerge(t *testing.T) {
	m := map[string]interface{}{}
	addUserData(m, "key", "init")
	require.Equal(t, 1, len(m))
	require.Equal(t, "init", m["key"])
}

func TestAddUserDataMerge(t *testing.T) {
	m := map[string]interface{}{"key": "before"}
	addUserData(m, "key", "init")
	require.Equal(t, 1, len(m))
	require.Equal(t, "before\ninit", m["key"])
}

func TestScanLocalFilesNoFiles(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	p, is := terraform.(*plugin)
	require.True(t, is)
	vms, err := p.scanLocalFiles()
	require.NoError(t, err)
	require.Empty(t, vms)
}

func TestScanLocalFilesInvalidFile(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	p, is := terraform.(*plugin)
	require.True(t, is)
	err := afero.WriteFile(p.fs, filepath.Join(p.Dir, "instance-12345.tf.json"), []byte("not-json"), 0644)
	require.NoError(t, err)
	_, err = p.scanLocalFiles()
	require.Error(t, err)
}

func TestScanLocalFilesNoVms(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	p, is := terraform.(*plugin)
	require.True(t, is)
	// Create a valid file without a VM type
	m := make(map[TResourceType]map[TResourceName]TResourceProperties)
	tformat := TFormat{Resource: m}
	buff, err := json.Marshal(tformat)
	require.NoError(t, err)
	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, "instance-12345.tf.json"), buff, 0644)
	require.NoError(t, err)
	_, err = p.scanLocalFiles()
	require.Error(t, err)
	require.Equal(t, "not found", err.Error())
}

func TestScanLocalFiles(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	p, is := terraform.(*plugin)
	require.True(t, is)

	// Create a few valid files, same type
	inst1 := make(map[TResourceType]map[TResourceName]TResourceProperties)
	inst1[VMSoftLayer] = map[TResourceName]TResourceProperties{
		"instance-12": {"key1": "val1"},
	}
	tformat := TFormat{Resource: inst1}
	buff, err := json.MarshalIndent(tformat, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, "instance-12.tf.json"), buff, 0644)
	require.NoError(t, err)

	inst2 := make(map[TResourceType]map[TResourceName]TResourceProperties)
	inst2[VMSoftLayer] = map[TResourceName]TResourceProperties{
		"instance-34": {"key2": "val2"},
	}
	tformat = TFormat{Resource: inst2}
	buff, err = json.MarshalIndent(tformat, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, "instance-34.tf.json"), buff, 0644)
	require.NoError(t, err)

	// And another type
	inst3 := make(map[TResourceType]map[TResourceName]TResourceProperties)
	inst3[VMAmazon] = map[TResourceName]TResourceProperties{
		"instance-56": {"key3": "val3"},
	}
	tformat = TFormat{Resource: inst3}
	buff, err = json.MarshalIndent(tformat, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, "instance-56.tf.json"), buff, 0644)
	require.NoError(t, err)

	// Should get 2 different resource types, 2 VMs for softlayer and 1 for AWS
	vms, err := p.scanLocalFiles()
	require.NoError(t, err)
	require.Equal(t, 2, len(vms))
	softlayerVMs, contains := vms[VMSoftLayer]
	require.True(t, contains)
	require.Equal(t, 2, len(softlayerVMs))
	require.Equal(t,
		softlayerVMs[TResourceName("instance-12")],
		TResourceProperties{"key1": "val1"},
	)
	require.Equal(t,
		softlayerVMs[TResourceName("instance-34")],
		TResourceProperties{"key2": "val2"},
	)
	awsVMs, contains := vms[VMAmazon]
	require.True(t, contains)
	require.Equal(t, 1, len(awsVMs))
	require.Equal(t,
		awsVMs[TResourceName("instance-56")],
		TResourceProperties{"key3": "val3"},
	)
}

func TestPlatformSpecificUpdatesNoProperties(t *testing.T) {
	platformSpecificUpdates(VMSoftLayer, "instance-1234", nil, nil)
}

func TestPlatformSpecificUpdatesWrongVMType(t *testing.T) {
	props := TResourceProperties{"key": "val"}
	// Azure does not have platform specific processing
	platformSpecificUpdates(VMAzure, "instance-1234", nil, props)
	require.Equal(t, 1, len(props))
	require.Equal(t, "val", props["key"])
}

func TestPlatformSpecificUpdatesAWSPrivateIPLogicalID(t *testing.T) {
	logicalID := instance.LogicalID("10.0.0.1")
	// private_ip set to logical ID address on AWS
	props := TResourceProperties{"private_ip": "INSTANCE_LOGICAL_ID"}
	platformSpecificUpdates(VMAmazon, "instance-1234", &logicalID, props)
	require.Equal(t,
		TResourceProperties{"private_ip": "10.0.0.1"},
		props)
	// but not on other platforms
	props = TResourceProperties{"private_ip": "INSTANCE_LOGICAL_ID"}
	platformSpecificUpdates(VMAzure, "instance-1234", &logicalID, props)
	require.Equal(t,
		TResourceProperties{"private_ip": "INSTANCE_LOGICAL_ID"},
		props)
}

func TestPlatformSpecificUpdatesAWSPrivateIPNoLogicalID(t *testing.T) {
	// private_ip removed if there is no logical ID
	props := TResourceProperties{"private_ip": "INSTANCE_LOGICAL_ID"}
	platformSpecificUpdates(VMAmazon, "instance-1234", nil, props)
	require.Equal(t, TResourceProperties{}, props)
}

func TestPlatformSpecificUpdatesNoHostnamePrefixNoLogicalID(t *testing.T) {
	props := TResourceProperties{}
	platformSpecificUpdates(VMSoftLayer, "instance-1234", nil, props)
	require.Equal(t, 1, len(props))
	require.Equal(t, "instance-1234", props["hostname"])
}

func TestPlatformSpecificUpdatesNoHostanmePrefixWithLogicalID(t *testing.T) {
	logicalID := instance.LogicalID("logical-id")
	props := TResourceProperties{}
	platformSpecificUpdates(VMSoftLayer, "instance-1234", &logicalID, props)
	require.Equal(t, 1, len(props))
	require.Equal(t, "logical-id", props["hostname"])
}

func TestPlatformSpecificUpdatesWithHostnamePrefixNoLogicalID(t *testing.T) {
	props := TResourceProperties{"@hostname_prefix": "prefix"}
	platformSpecificUpdates(VMSoftLayer, "instance-1234", nil, props)
	require.Equal(t, 1, len(props))
	require.Equal(t, "prefix-1234", props["hostname"])
}

func TestPlatformSpecificUpdatesWithHostnamePrefixWithLogicalID(t *testing.T) {
	logicalID := instance.LogicalID("logical-id")
	props := TResourceProperties{"@hostname_prefix": "prefix"}
	platformSpecificUpdates(VMSoftLayer, "instance-1234", &logicalID, props)
	require.Equal(t, 1, len(props))
	require.Equal(t, "prefix-logical-id", props["hostname"])
}

func TestPlatformSpecificUpdatesWithNonStringHostnamePrefix(t *testing.T) {
	logicalID := instance.LogicalID("logical-id")
	props := TResourceProperties{"@hostname_prefix": 1, "hostname": "hostname"}
	platformSpecificUpdates(VMSoftLayer, "instance-1234", &logicalID, props)
	require.Equal(t, 1, len(props))
	require.Equal(t, "logical-id", props["hostname"])
}

func TestPlatformSpecificUpdatesWithEmptyHostanmePrefix(t *testing.T) {
	props := TResourceProperties{"@hostname_prefix": "", "hostname": "hostname"}
	platformSpecificUpdates(VMSoftLayer, "instance-1234", nil, props)
	require.Equal(t, 1, len(props))
	require.Equal(t, "instance-1234", props["hostname"])
}

func TestRenderInstIDVarNoReplace(t *testing.T) {
	result, err := renderInstIDVar("{}", instance.ID("id"))
	require.NoError(t, err)
	require.Equal(t, "{}", result)
}

func TestRenderInstIDVar(t *testing.T) {
	input := `{
 "id": "{{ var "/self/instId" }}",
 "key": "val"
}`
	expected := `{
 "id": "id",
 "key": "val"
}`
	result, err := renderInstIDVar(input, instance.ID("id"))
	require.NoError(t, err)
	require.JSONEq(t, expected, result)
}

func TestLabelNoFiles(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	p, is := terraform.(*plugin)
	require.True(t, is)
	err := p.Label(instance.ID("ID"), nil)
	require.Error(t, err)
}

func TestLabelInvalidFile(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	p, is := terraform.(*plugin)
	require.True(t, is)
	id := "instance-1234"
	err := afero.WriteFile(p.fs, filepath.Join(p.Dir, fmt.Sprintf("%v.tf.json", id)), []byte("not-json"), 0644)
	require.NoError(t, err)
	err = p.Label(instance.ID(id), nil)
	require.Error(t, err)
}

func TestLabelNoVM(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	p, is := terraform.(*plugin)
	require.True(t, is)
	id := "instance-1234"
	// No VM data in instance definition
	inst := make(map[TResourceType]map[TResourceName]TResourceProperties)
	tformat := TFormat{Resource: inst}
	buff, err := json.MarshalIndent(tformat, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, fmt.Sprintf("%v.tf.json", id)), buff, 0644)
	require.NoError(t, err)
	err = p.Label(instance.ID(id), nil)
	require.Error(t, err)
	require.Equal(t, "not found", err.Error())
}

func TestLabelNoProperties(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	p, is := terraform.(*plugin)
	require.True(t, is)
	id := "instance-1234"
	// Resource does not have any properties
	inst := make(map[TResourceType]map[TResourceName]TResourceProperties)
	inst[VMSoftLayer] = map[TResourceName]TResourceProperties{"instance-1234": {}}
	tformat := TFormat{Resource: inst}
	buff, err := json.MarshalIndent(tformat, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, fmt.Sprintf("%v.tf.json", id)), buff, 0644)
	require.NoError(t, err)
	err = p.Label(instance.ID(id), nil)
	require.Error(t, err)
	require.Equal(t, "not found:instance-1234", err.Error())
}

func TestLabelCreateNewTags(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	p, is := terraform.(*plugin)
	require.True(t, is)

	// Create a file without any tags for each VMType
	for index, vmType := range VMTypes {
		inst := make(map[TResourceType]map[TResourceName]TResourceProperties)
		id := fmt.Sprintf("instance-%v", index)
		key := vmType.(TResourceType)
		inst[key] = map[TResourceName]TResourceProperties{
			TResourceName(id): {
				fmt.Sprintf("key-%v", index): fmt.Sprintf("val-%v", index),
			},
		}
		tformat := TFormat{Resource: inst}
		buff, err := json.MarshalIndent(tformat, " ", " ")
		require.NoError(t, err)
		err = afero.WriteFile(p.fs, filepath.Join(p.Dir, fmt.Sprintf("%v.tf.json", id)), buff, 0644)
		require.NoError(t, err)
	}

	// Add some labels
	labels := map[string]string{
		"label1": "value1",
		"label2": "value2",
	}
	for index := range VMTypes {
		id := fmt.Sprintf("instance-%v", index)
		err := p.Label(instance.ID(id), labels)
		require.NoError(t, err)
	}

	// Verify that the labels were added
	for index, vmType := range VMTypes {
		id := fmt.Sprintf("instance-%v", index)
		buff, err := ioutil.ReadFile(filepath.Join(p.Dir, id+".tf.json"))
		require.NoError(t, err)
		tf := TFormat{}
		err = types.AnyBytes(buff).Decode(&tf)
		require.NoError(t, err)
		actualVMType, vmName, props, err := FindVM(&tf)
		require.NoError(t, err)
		require.Equal(t, vmType, actualVMType)
		require.Equal(t, TResourceName(id), vmName)
		_, contains := props["tags"]
		require.True(t, contains)
		if vmType == VMSoftLayer {
			// Tags as list
			expectedTags := []string{"label1:value1", "label2:value2"}
			actualTags := []string{}
			for _, tag := range props["tags"].([]interface{}) {
				actualTags = append(actualTags, tag.(string))
			}
			sort.Strings(actualTags)
			require.Equal(t, expectedTags, actualTags)
		} else {
			// Tags are map
			require.Equal(t,
				map[string]interface{}{
					"label1": "value1",
					"label2": "value2",
				},
				props["tags"],
			)
		}
	}
}

func TestLabelMergeTags(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	p, is := terraform.(*plugin)
	require.True(t, is)

	// Create a file with existing tags for each VMType
	for index, vmType := range VMTypes {
		inst := make(map[TResourceType]map[TResourceName]TResourceProperties)
		id := fmt.Sprintf("instance-%v", index)
		key := vmType.(TResourceType)
		var tags interface{}
		if vmType == VMSoftLayer {
			tags = []string{"tag1:val1", "tag2:val2"}
		} else {
			tags = map[string]string{"tag1": "val1", "tag2": "val2"}
		}
		inst[key] = map[TResourceName]TResourceProperties{
			TResourceName(id): {
				fmt.Sprintf("key-%v", index): fmt.Sprintf("val-%v", index),
				"tags": tags,
			},
		}
		tformat := TFormat{Resource: inst}
		buff, err := json.MarshalIndent(tformat, " ", " ")
		require.NoError(t, err)
		err = afero.WriteFile(p.fs, filepath.Join(p.Dir, fmt.Sprintf("%v.tf.json", id)), buff, 0644)
		require.NoError(t, err)
	}

	// Add some labels
	labels := map[string]string{
		"label1": "value1",
		"label2": "value2",
	}
	for index := range VMTypes {
		id := fmt.Sprintf("instance-%v", index)
		err := p.Label(instance.ID(id), labels)
		require.NoError(t, err)
	}

	// Verify that the labels were added
	for index, vmType := range VMTypes {
		id := fmt.Sprintf("instance-%v", index)
		buff, err := ioutil.ReadFile(filepath.Join(p.Dir, id+".tf.json"))
		require.NoError(t, err)
		tf := TFormat{}
		err = types.AnyBytes(buff).Decode(&tf)
		require.NoError(t, err)
		actualVMType, vmName, props, err := FindVM(&tf)
		require.NoError(t, err)
		require.Equal(t, vmType, actualVMType)
		require.Equal(t, TResourceName(id), vmName)
		_, contains := props["tags"]
		require.True(t, contains)
		if vmType == VMSoftLayer {
			// Tags as list
			expectedTags := []string{"label1:value1", "label2:value2", "tag1:val1", "tag2:val2"}
			actualTags := []string{}
			for _, tag := range props["tags"].([]interface{}) {
				actualTags = append(actualTags, tag.(string))
			}
			sort.Strings(actualTags)
			require.Equal(t, expectedTags, actualTags)
		} else {
			// Tags are map
			require.Equal(t,
				map[string]interface{}{
					"tag1":   "val1",
					"tag2":   "val2",
					"label1": "value1",
					"label2": "value2",
				},
				props["tags"],
			)
		}
	}
}

func TestTerraformTagsEmpty(t *testing.T) {
	// No tags
	props := TResourceProperties{"foo": "bar"}
	result := terraformTags(props, "tags")
	require.Equal(t, map[string]string{}, result)
	// Invalid type
	props = TResourceProperties{
		"foo":  "bar",
		"tags": true,
	}
	result = terraformTags(props, "tags")
	require.Equal(t, map[string]string{}, result)
}

func TestTerraformTagsMap(t *testing.T) {
	props := TResourceProperties{
		"foo": "bar",
		"tags": map[string]interface{}{
			"t1": "v1",
			"t2": "v2",
			"t3": "v3:extra",
		},
	}
	result := terraformTags(props, "tags")
	require.Equal(t,
		map[string]string{"t1": "v1", "t2": "v2", "t3": "v3:extra"},
		result,
	)
}

func TestTerraformTagsList(t *testing.T) {
	props := TResourceProperties{
		"foo": "bar",
		"tags": []interface{}{
			"t1:v1",
			"t2:v2",
			"t3:v3:extra",
		},
	}
	result := terraformTags(props, "tags")
	require.Equal(t,
		map[string]string{"t1": "v1", "t2": "v2", "t3": "v3:extra"},
		result,
	)
}

func TestTerraformTagRawProperties(t *testing.T) {
	props := TResourceProperties{
		"foo":     "bar",
		"tags.%":  2,
		"tags.t1": "v1",
		"tags.t2": "v2",
	}
	result := terraformTags(props, "tags")
	require.Equal(t,
		map[string]string{"t1": "v1", "t2": "v2"},
		result,
	)
}

func TestTerraformLogicalIDNoID(t *testing.T) {
	// As map
	props := TResourceProperties{"tags": map[string]string{}}
	id := terraformLogicalID(props)
	require.Nil(t, id)
	// As list
	props = TResourceProperties{"tags": []interface{}{}}
	id = terraformLogicalID(props)
	require.Nil(t, id)
	// Invalid type
	props = TResourceProperties{"tags": true}
	id = terraformLogicalID(props)
	require.Nil(t, id)
}

func TestTerraformLogicalIDFromMap(t *testing.T) {
	props := TResourceProperties{
		"tags": map[string]interface{}{
			"foo":       "bar",
			"lOGiCALid": "logical-id",
		},
	}
	id := terraformLogicalID(props)
	require.Equal(t, instance.LogicalID("logical-id"), *id)
}

func TestTerraformLogicalIDFromList(t *testing.T) {
	props := TResourceProperties{
		"tags": []interface{}{
			"foo:bar",
			"lOGiCALid:logical-id:val",
		},
	}
	id := terraformLogicalID(props)
	require.Equal(t, instance.LogicalID("logical-id:val"), *id)
}

func TestDestroyInstanceNotExists(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	err := terraform.Destroy(instance.ID("id"), instance.Termination)
	require.Error(t, err)
}

func TestDestroy(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	p, is := terraform.(*plugin)
	require.True(t, is)
	id := "instance-1234"
	inst := make(map[TResourceType]map[TResourceName]TResourceProperties)
	tformat := TFormat{Resource: inst}
	buff, err := json.MarshalIndent(tformat, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, fmt.Sprintf("%v.tf.json", id)), buff, 0644)
	require.NoError(t, err)
	result := terraform.Destroy(instance.ID(id), instance.Termination)
	require.Nil(t, result)
	// The file has been removed
	files, err := ioutil.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 0)
}

func TestDescribeNoFiles(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	results, err := terraform.DescribeInstances(map[string]string{}, false)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{}, results)
}

func TestDescribe(t *testing.T) {
	terraform, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	p, is := terraform.(*plugin)
	require.True(t, is)

	// Instance1, unique tag and shared tag
	inst1 := make(map[TResourceType]map[TResourceName]TResourceProperties)
	id1 := "instance-1"
	tags1 := []string{"tag1:val1", "tagShared:valShared"}
	inst1[VMSoftLayer] = map[TResourceName]TResourceProperties{
		TResourceName(id1): {"tags": tags1},
	}
	buff, err := json.MarshalIndent(TFormat{Resource: inst1}, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, fmt.Sprintf("%v.tf.json", id1)), buff, 0644)
	require.NoError(t, err)
	// Instance1, unique tag and shared tag
	inst2 := make(map[TResourceType]map[TResourceName]TResourceProperties)
	id2 := "instance-2"
	tags2 := map[string]string{"tag2": "val2", "tagShared": "valShared"}
	inst2[VMAzure] = map[TResourceName]TResourceProperties{
		TResourceName(id2): {"tags": tags2},
	}
	buff, err = json.MarshalIndent(TFormat{Resource: inst2}, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, fmt.Sprintf("%v.tf.json", id2)), buff, 0644)
	require.NoError(t, err)
	// Instance1, unique tag only
	inst3 := make(map[TResourceType]map[TResourceName]TResourceProperties)
	id3 := "instance-3"
	tags3 := map[string]string{"tag3": "val3"}
	inst3[VMAmazon] = map[TResourceName]TResourceProperties{
		TResourceName(id3): {"tags": tags3},
	}
	buff, err = json.MarshalIndent(TFormat{Resource: inst3}, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, fmt.Sprintf("%v.tf.json", id3)), buff, 0644)
	require.NoError(t, err)

	// First instance matches
	results, err := terraform.DescribeInstances(
		map[string]string{"tag1": "val1"},
		false)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, instance.ID(id1), results[0].ID)
	results, err = terraform.DescribeInstances(
		map[string]string{"tag1": "val1", "tagShared": "valShared"},
		false)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, instance.ID(id1), results[0].ID)

	// Second instance matches
	results, err = terraform.DescribeInstances(
		map[string]string{"tag2": "val2"},
		false)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, instance.ID(id2), results[0].ID)

	// Both instances matches
	results, err = terraform.DescribeInstances(
		map[string]string{"tagShared": "valShared"},
		false)
	require.NoError(t, err)
	require.Equal(t, 2, len(results))
	var ids []instance.ID
	for _, result := range results {
		ids = append(ids, result.ID)
	}
	require.Contains(t, ids, instance.ID(id1))
	require.Contains(t, ids, instance.ID(id2))

	// No instances match
	results, err = terraform.DescribeInstances(
		map[string]string{"tag1": "val1", "tagShared": "valShared", "foo": "bar"},
		false)
	require.NoError(t, err)
	require.Empty(t, results)
	results, err = terraform.DescribeInstances(
		map[string]string{"TAG2": "val2"},
		false)
	require.NoError(t, err)
	require.Empty(t, results)

	// All instances match (no tags passed)
	results, err = terraform.DescribeInstances(map[string]string{}, false)
	require.NoError(t, err)
	require.Equal(t, 3, len(results))
	ids = []instance.ID{}
	for _, result := range results {
		ids = append(ids, result.ID)
	}
	require.Contains(t, ids, instance.ID(id1))
	require.Contains(t, ids, instance.ID(id2))
	require.Contains(t, ids, instance.ID(id3))
}

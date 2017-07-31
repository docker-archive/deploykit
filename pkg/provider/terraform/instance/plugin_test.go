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
	"time"

	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestProcessBootstrapErrors(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	// Group spec but no instance ID
	groupSpecURL := "str://foo"
	instID := ""
	b := bootstrapOptions{GroupSpecURL: &groupSpecURL, InstanceID: &instID}
	err := tf.processBootstrap(&b)
	require.Error(t, err)
	require.Equal(t,
		"Bootstrap instance ID required with bootstrap group spec",
		err.Error())
	// No group spec but instance ID
	groupSpecURL = ""
	instID = "1234"
	b = bootstrapOptions{GroupSpecURL: &groupSpecURL, InstanceID: &instID}
	err = tf.processBootstrap(&b)
	require.Error(t, err)
	require.Equal(t,
		"Bootstrap group spec required with bootstrap instance ID",
		err.Error())
	// Invalid group spec template with instance ID
	groupSpecURL = "str://{{ nosuchfn }}"
	instID = "1234"
	b = bootstrapOptions{GroupSpecURL: &groupSpecURL, InstanceID: &instID}
	err = tf.processBootstrap(&b)
	require.Error(t, err)
}

// getPlugin returns the terraform instance plugin to use for testing and the
// directory where the .tf.json files should be stored
func getPlugin(t *testing.T) (*plugin, string) {
	dir, err := ioutil.TempDir("", "infrakit-instance-terraform")
	require.NoError(t, err)
	tf := NewTerraformInstancePlugin(dir, 1*time.Second, false, nil)
	tf.(*plugin).pretend = true
	p, is := tf.(*plugin)
	require.True(t, is)
	return p, dir
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
		if vmType == VMSoftLayer || vmType == VMIBMCloud {
			tags := props["tags"]
			require.Len(t, tags, 2)
			// Note that tags are all lowercase
			require.Contains(t, tags, "logicalid:logical-id-1")
			require.Contains(t, tags, "name:instance-1234")
		} else {
			expectedTags := map[string]interface{}{
				"LogicalID": "logical-id-1",
				"Name":      "instance-1234",
			}
			require.Equal(t, expectedTags, props["tags"])
		}
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
		if vmType == VMSoftLayer || vmType == VMIBMCloud {
			expectedTags = []interface{}{"name:instance-1234"}
		} else {
			expectedTags = map[string]interface{}{"Name": "instance-1234"}
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
		if vmType == VMSoftLayer || vmType == VMIBMCloud {
			tags := props["tags"]
			require.Len(t, tags, 3)
			// Note that tags are all lowercase
			require.Contains(t, tags, "foo:bar")
			require.Contains(t, tags, "logicalid:logical-id-1")
			require.Contains(t, tags, "name:existing-name")
		} else {
			expectedTags := map[string]interface{}{
				"LogicalID": "logical-id-1",
				"name":      "existing-name",
				"foo":       "bar",
			}
			require.Equal(t, expectedTags, props["tags"])
		}
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
		if vmType == VMSoftLayer || vmType == VMIBMCloud {
			tags := props["tags"]
			require.Len(t, tags, 2)
			require.Contains(t, tags, "foo:bar")
			require.Contains(t, tags, "name:existing-name")
		} else {
			expectedTags := map[string]interface{}{
				"Name": "existing-name",
				"foo":  "bar",
			}
			require.Equal(t, expectedTags, props["tags"])
		}
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
		case VMSoftLayer, VMIBMCloud:
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
		case VMSoftLayer, VMIBMCloud:
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
		case VMSoftLayer, VMIBMCloud:
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
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	spec := instance.Spec{
		Properties:  types.AnyString("{}"),
		Tags:        map[string]string{},
		Init:        "",
		Attachments: []instance.Attachment{},
		LogicalID:   nil,
	}
	_, err := tf.Provision(spec)
	require.Error(t, err)
	require.Equal(t, "no resource section", err.Error())
}

func TestProvisionNoVM(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	spec := instance.Spec{
		Properties:  types.AnyString("{\"resource\": {}}"),
		Tags:        map[string]string{},
		Init:        "",
		Attachments: []instance.Attachment{},
		LogicalID:   nil,
	}
	_, err := tf.Provision(spec)
	require.Error(t, err)
	require.Equal(t, "not found", err.Error())
}

func TestProvisionNoVMProperties(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	spec := instance.Spec{
		Properties:  types.AnyString("{\"resource\": {\"aws_instance\": {}}}"),
		Tags:        map[string]string{},
		Init:        "",
		Attachments: []instance.Attachment{},
		LogicalID:   nil,
	}
	_, err := tf.Provision(spec)
	require.Error(t, err)
	require.Equal(t, "no-vm-instance-in-spec", err.Error())
}

func TestProvisionInvalidTemplateProperties(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	spec := instance.Spec{
		Properties:  types.AnyString("{{}"),
		Tags:        map[string]string{},
		Init:        "",
		Attachments: []instance.Attachment{},
		LogicalID:   nil,
	}
	_, err := tf.Provision(spec)
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "template:"))
}

func TestProvisionInvalidTemplateInit(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	spec := instance.Spec{
		Properties:  types.AnyString("{}"),
		Tags:        map[string]string{},
		Init:        "{{}",
		Attachments: []instance.Attachment{},
		LogicalID:   nil,
	}
	_, err := tf.Provision(spec)
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "template:"))
}

func TestProvisionDescribeDestroyScope(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	m := map[TResourceType]map[TResourceName]TResourceProperties{
		VMAmazon: {
			TResourceName("host"): {
				"vmp1": "vmv1", "vmp2": "vmv2",
				PropScope: ValScopeDefault,
			},
		},
		TResourceType("softlayer_file_storage"): {
			TResourceName("worker_fs"): {
				"fsp1": "fsv1", "fsp2": "fsv2",
				PropScope: ValScopeDedicated,
			},
		},
		TResourceType("softlayer_block_storage"): {
			TResourceName("worker_bs"): {
				"bsp1": "bsv1", "bsp2": "bsv2",
				PropScope: "managers",
			},
		},
		TResourceType("another-dedicated"): {
			TResourceName("another-dedicated-name"): {
				"kded-1":  "vded-1",
				PropScope: ValScopeDedicated,
			},
		},
		TResourceType("another-default"): {
			TResourceName("another-default-name"): {"kdef-1": "vdef-1"},
		},
	}
	tformat := TFormat{Resource: m}
	buff, err := json.MarshalIndent(tformat, "  ", "  ")
	require.NoError(t, err)
	// Issue 2 provisions; should get dedicated for both and a single global
	id1, err := tf.Provision(instance.Spec{
		Properties: types.AnyBytes(buff),
		Tags:       map[string]string{"tag1": "val1"},
	})
	require.NoError(t, err)
	id2, err := tf.Provision(instance.Spec{
		Properties: types.AnyBytes(buff),
		Tags:       map[string]string{"tag1": "val1"},
	})
	require.NoError(t, err)
	results, err := tf.DescribeInstances(
		map[string]string{"tag1": "val1"},
		false,
	)
	require.NoError(t, err)
	require.Len(t, results, 2)
	expectedAttach1 := []string{string(*id1) + "-dedicated", "scope-managers"}
	require.Contains(t,
		results,
		instance.Description{
			ID: *id1,
			Tags: map[string]string{
				attachTag: strings.Join(expectedAttach1, ","),
				"Name":    string(*id1),
				"tag1":    "val1",
			},
		})
	expectedAttach2 := []string{string(*id2) + "-dedicated", "scope-managers"}
	require.Contains(t,
		results,
		instance.Description{
			ID: *id2,
			Tags: map[string]string{
				attachTag: strings.Join(expectedAttach2, ","),
				"Name":    string(*id2),
				"tag1":    "val1",
			},
		})
	// Should be files for:
	// 2 VMs
	// 2 dedicated
	// 1 global ("managers" scope)
	files, err := ioutil.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 5)
	expectedPaths := []string{
		expectedAttach1[0],
		expectedAttach2[0],
		string(*id1),
		string(*id2),
		"scope-managers",
	}
	for _, path := range expectedPaths {
		tfPath1 := filepath.Join(dir, path+".tf.json")
		_, err = ioutil.ReadFile(tfPath1)
		require.NoError(t, err, fmt.Sprintf("Expected path %s does not exist", path))
	}
	// Should be able to Destroy the first VM and the dedicated file should be removed
	err = tf.Destroy(*id1, instance.Termination)
	require.NoError(t, err)
	files, err = ioutil.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 3)
	expectedPaths = []string{
		expectedAttach2[0],
		string(*id2),
		"scope-managers",
	}
	for _, path := range expectedPaths {
		tfPath1 := filepath.Join(dir, path+".tf.json")
		_, err = ioutil.ReadFile(tfPath1)
		require.NoError(t, err, fmt.Sprintf("Expected path %s does not exist", path))
	}
	// Destroying the second VM should remove everything
	err = tf.Destroy(*id2, instance.Termination)
	require.NoError(t, err)
	files, err = ioutil.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 0)
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
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	config := types.AnyString(properties)
	err := tf.Validate(config)
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
	id1, err := tf.Provision(instanceSpec1)
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
	id2, err := tf.Provision(instanceSpec2)
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
		case VMSoftLayer, VMIBMCloud:
			require.Equal(t, conv([]interface{}{
				"terraform_demo_swarm_mgr_sl",
				"label1:value1",
				"label2:value2",
				"name:" + string(*id2),
				"logicalid:logical:id-2",
			}), conv(props["tags"].([]interface{})))
			require.Equal(t, expectedUserData2, props["user_metadata"])

			// If a hostname was specified, the expectation is that the hostname is appended with the logical ID
			if value[PropHostnamePrefix] != nil && strings.Trim(value[PropHostnamePrefix].(string), " ") != "" {
				expectedHostname := "softlayer-hostname-logical:id-2"
				require.Equal(t, expectedHostname, props["hostname"])
			} else {
				// If no hostname was specified, the hostname should equal the logical ID
				require.Equal(t, "logical:id-2", props["hostname"])
			}
			// Verify the hostname prefix key/value is no longer in the props
			require.Nil(t, props[PropHostnamePrefix])

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
	case VMSoftLayer, VMIBMCloud:
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
	list, err := tf.DescribeInstances(map[string]string{"label1": "value1"}, false)
	require.NoError(t, err)
	require.Contains(t, list, inst1)
	require.Contains(t, list, inst2)

	// re-label instance2
	err = tf.Label(*id2, map[string]string{
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
	case VMSoftLayer, VMIBMCloud:
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
	list, err = tf.DescribeInstances(map[string]string{"label1": "changed1"}, false)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{inst2}, list)

	// Only a single match: label1=value1
	list, err = tf.DescribeInstances(map[string]string{"label1": "value1"}, false)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{inst1}, list)

	// No matches: label1=foo
	list, err = tf.DescribeInstances(map[string]string{"label1": "foo"}, false)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{}, list)

	// Destroy, then none should match and 1 file should be removed
	err = tf.Destroy(*id2, instance.Termination)
	require.NoError(t, err)
	files, err := ioutil.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, filepath.Base(tfPath1), files[0].Name())

	list, err = tf.DescribeInstances(map[string]string{"label1": "changed1"}, false)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{}, list)

	err = tf.Destroy(*id1, instance.Termination)
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
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	config := types.AnyString("not-going-to-decode")
	err := tf.Validate(config)
	require.Error(t, err)
}

func TestValidate(t *testing.T) {
	tf, dir := getPlugin(t)
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
	err = tf.Validate(types.AnyBytes(config))
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
	err = tf.Validate(types.AnyBytes(config))
	require.NoError(t, err)
	// And pass with 0
	delete(props["resource"], VMSoftLayer)
	require.Empty(t, props["resource"])
	config, err = json.Marshal(props)
	require.NoError(t, err)
	err = tf.Validate(types.AnyBytes(config))
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

func TestWriteTerraformFilesError(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	tformat := TFormat{Resource: map[TResourceType]map[TResourceName]TResourceProperties{
		VMSoftLayer: {"host": {}}},
	}
	// Before writing the file delete the directory to create an error
	os.RemoveAll(dir)
	err := tf.writeTerraformFiles(nil, "instance-1234", &tformat, VMSoftLayer, TResourceProperties{})
	require.Error(t, err)
}

func TestWriteTerraformFilesVMOnly(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	name := "instance-1234"
	m := map[TResourceType]map[TResourceName]TResourceProperties{
		VMSoftLayer: {
			TResourceName("host"): {"p1": "v1", "p2": "v2"},
		},
	}
	tformat := TFormat{Resource: m}
	err := tf.writeTerraformFiles(nil, name, &tformat, VMSoftLayer, TResourceProperties{"p3": "v3"})
	require.NoError(t, err)
	// Read single file from disk
	files, err := ioutil.ReadDir(tf.Dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	buff, err := ioutil.ReadFile(filepath.Join(tf.Dir, name+".tf.json"))
	require.NoError(t, err)
	tFormat := TFormat{}
	err = types.AnyBytes(buff).Decode(&tFormat)
	require.NoError(t, err)
	// 1 resource type
	require.Len(t, tFormat.Resource, 1)
	require.Equal(t,
		map[TResourceName]TResourceProperties{TResourceName(name): {
			"hostname": "instance-1234",
			"p3":       "v3"}},
		tFormat.Resource[VMSoftLayer],
	)
}

func TestWriteTerraformFilesVMOnlyLogicalId(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	logicalID := instance.LogicalID("mgr1")
	name := "instance-1234"
	m := map[TResourceType]map[TResourceName]TResourceProperties{
		VMSoftLayer: {
			TResourceName("host"): {"p1": "v1", "p2": "v2"},
		},
	}
	tformat := TFormat{Resource: m}
	err := tf.writeTerraformFiles(&logicalID, name, &tformat, VMSoftLayer, TResourceProperties{"p3": "v3"})
	require.NoError(t, err)
	// Read single file from disk
	files, err := ioutil.ReadDir(tf.Dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	buff, err := ioutil.ReadFile(filepath.Join(tf.Dir, name+".tf.json"))
	require.NoError(t, err)
	tFormat := TFormat{}
	err = types.AnyBytes(buff).Decode(&tFormat)
	require.NoError(t, err)
	// 1 resource type
	require.Len(t, tFormat.Resource, 1)
	require.Equal(t,
		map[TResourceName]TResourceProperties{TResourceName(name): {
			"hostname": "mgr1",
			"p3":       "v3"}},
		tFormat.Resource[VMSoftLayer],
	)
}

func TestWriteTerraformFilesMultipleDefaultResources(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	name := "instance-1234"
	m := map[TResourceType]map[TResourceName]TResourceProperties{
		VMSoftLayer: {
			TResourceName("host"): {"vmp1": "vmv1", "vmp2": "vmv2"},
		},
		TResourceType("softlayer_file_storage"): {
			TResourceName("worker_fs"): {"fsp1": "fsv1", "fsp2": "fsv2"},
		},
		TResourceType("softlayer_block_storage"): {
			TResourceName("worker_bs"): {"bsp1": "bsv1", "bsp2": "bsv2"},
		},
	}
	tformat := TFormat{Resource: m}
	err := tf.writeTerraformFiles(nil, name, &tformat, VMSoftLayer, TResourceProperties{"vmp3": "vmv3"})
	require.NoError(t, err)
	// Read single file from disk
	files, err := ioutil.ReadDir(tf.Dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	buff, err := ioutil.ReadFile(filepath.Join(tf.Dir, name+".tf.json"))
	require.NoError(t, err)
	tFormat := TFormat{}
	err = types.AnyBytes(buff).Decode(&tFormat)
	require.NoError(t, err)
	// 3 resource type
	require.Len(t, tFormat.Resource, 3)
	// VM resource
	vmType := tFormat.Resource[VMSoftLayer]
	require.NotNil(t, vmType)
	require.Equal(t,
		map[TResourceName]TResourceProperties{
			TResourceName(name): {
				"hostname": name,
				"vmp3":     "vmv3",
			},
		},
		vmType,
	)
	// File storage
	fsType := tFormat.Resource[TResourceType("softlayer_file_storage")]
	require.NotNil(t, fsType)
	require.Equal(t,
		map[TResourceName]TResourceProperties{
			TResourceName(name + "-worker_fs"): {"fsp1": "fsv1", "fsp2": "fsv2"},
		},
		fsType,
	)
	// Block storage
	bsType := tFormat.Resource[TResourceType("softlayer_block_storage")]
	require.NotNil(t, bsType)
	require.Equal(t,
		map[TResourceName]TResourceProperties{
			TResourceName(name + "-worker_bs"): {"bsp1": "bsv1", "bsp2": "bsv2"},
		},
		bsType,
	)
}

func TestWriteTerraformFilesMultipleResourcesScopeTypes(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	name := "instance-1234"
	globalName := "managers"
	m := map[TResourceType]map[TResourceName]TResourceProperties{
		VMAmazon: {
			TResourceName("host"): {
				"vmp1": "vmv1", "vmp2": "vmv2",
				PropScope: ValScopeDefault,
			},
		},
		TResourceType("softlayer_file_storage"): {
			TResourceName("worker_fs"): {
				"fsp1": "fsv1", "fsp2": "fsv2",
				PropScope: ValScopeDedicated,
			},
		},
		TResourceType("softlayer_block_storage"): {
			TResourceName("worker_bs"): {
				"bsp1": "bsv1", "bsp2": "bsv2",
				PropScope: globalName,
			},
		},
		TResourceType("another-dedicated"): {
			TResourceName("another-dedicated-name"): {
				"kded-1":  "vded-1",
				PropScope: ValScopeDedicated,
			},
		},
		TResourceType("another-default"): {
			TResourceName("another-default-name"): {"kdef-1": "vdef-1"},
		},
	}
	tformat := TFormat{Resource: m}
	err := tf.writeTerraformFiles(nil,
		name,
		&tformat,
		VMAmazon,
		TResourceProperties{"vmp3": "vmv3"})
	require.NoError(t, err)
	// Should be 3 files on disk
	files, err := ioutil.ReadDir(tf.Dir)
	require.NoError(t, err)
	require.Len(t, files, 3)
	filenames := []string{}
	for _, file := range files {
		filenames = append(filenames, file.Name())
	}
	require.Contains(t, filenames, fmt.Sprintf("%s.tf.json", name))
	require.Contains(t, filenames, fmt.Sprintf("%s-dedicated.tf.json", name))
	expectedGlobalFilename := fmt.Sprintf("scope-%s.tf.json", globalName)
	require.Contains(t, filenames, expectedGlobalFilename)
	// Default
	buff, err := ioutil.ReadFile(filepath.Join(tf.Dir, fmt.Sprintf("%s.tf.json", name)))
	require.NoError(t, err)
	tFormat := TFormat{}
	err = types.AnyBytes(buff).Decode(&tFormat)
	require.NoError(t, err)
	require.Equal(t,
		map[TResourceType]map[TResourceName]TResourceProperties{
			VMAmazon: {
				TResourceName(name): {
					"vmp3": "vmv3",
					"tags": map[string]interface{}{
						attachTag: fmt.Sprintf("%s-dedicated,%s", name, "scope-managers"),
					},
				},
			},
			TResourceType("another-default"): {
				TResourceName(name + "-another-default-name"): {"kdef-1": "vdef-1"},
			},
		},
		tFormat.Resource,
	)
	// Dedicated
	buff, err = ioutil.ReadFile(filepath.Join(tf.Dir, fmt.Sprintf("%s-dedicated.tf.json", name)))
	require.NoError(t, err)
	tFormat = TFormat{}
	err = types.AnyBytes(buff).Decode(&tFormat)
	require.NoError(t, err)
	require.Equal(t,
		map[TResourceType]map[TResourceName]TResourceProperties{
			TResourceType("softlayer_file_storage"): {
				TResourceName(name + "-worker_fs"): {"fsp1": "fsv1", "fsp2": "fsv2"},
			},
			TResourceType("another-dedicated"): {
				TResourceName(name + "-another-dedicated-name"): {"kded-1": "vded-1"},
			},
		},
		tFormat.Resource,
	)
	// Global
	buff, err = ioutil.ReadFile(filepath.Join(tf.Dir, expectedGlobalFilename))
	require.NoError(t, err)
	tFormat = TFormat{}
	err = types.AnyBytes(buff).Decode(&tFormat)
	require.NoError(t, err)
	require.Equal(t,
		map[TResourceType]map[TResourceName]TResourceProperties{
			TResourceType("softlayer_block_storage"): {
				TResourceName(globalName + "-worker_bs"): {"bsp1": "bsv1", "bsp2": "bsv2"},
			},
		},
		tFormat.Resource,
	)
}

func TestWriteTerraformFilesMultipleResourcesDedicatedScope(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	logicalID := instance.LogicalID("mgr1")
	name := "instance-1234"
	m := map[TResourceType]map[TResourceName]TResourceProperties{
		VMSoftLayer: {
			TResourceName("host"): {"vmp1": "vmv1", "vmp2": "vmv2"},
		},
		TResourceType("softlayer_file_storage"): {
			TResourceName("worker_fs"): {
				"fsp1": "fsv1", "fsp2": "fsv2",
				PropScope: ValScopeDedicated,
			},
		},
		TResourceType("softlayer_block_storage"): {
			TResourceName("worker_bs"): {
				"bsp1": "bsv1", "bsp2": "bsv2",
				PropScope: ValScopeDedicated,
			},
		},
	}
	tformat := TFormat{Resource: m}
	err := tf.writeTerraformFiles(&logicalID,
		name,
		&tformat,
		VMSoftLayer,
		TResourceProperties{"vmp3": "vmv3"})
	require.NoError(t, err)
	// Should be 2 files on disk
	files, err := ioutil.ReadDir(tf.Dir)
	require.NoError(t, err)
	require.Len(t, files, 2)
	filenames := []string{}
	for _, file := range files {
		filenames = append(filenames, file.Name())
	}
	require.Contains(t, filenames, fmt.Sprintf("%s.tf.json", name))
	require.Contains(t, filenames, fmt.Sprintf("%s-dedicated.tf.json", name))
	// VM file
	buff, err := ioutil.ReadFile(filepath.Join(tf.Dir, name+".tf.json"))
	require.NoError(t, err)
	tFormat := TFormat{}
	err = types.AnyBytes(buff).Decode(&tFormat)
	require.NoError(t, err)
	require.Len(t, tFormat.Resource, 1)
	vmType := tFormat.Resource[VMSoftLayer]
	require.Equal(t,
		map[TResourceName]TResourceProperties{
			TResourceName(name): {
				"hostname": "mgr1",
				"vmp3":     "vmv3",
				"tags": []interface{}{
					fmt.Sprintf("%s:%s-dedicated", attachTag, name),
				},
			},
		},
		vmType,
	)
	// File storage and block storage
	buff, err = ioutil.ReadFile(filepath.Join(tf.Dir, fmt.Sprintf("%s-dedicated.tf.json", name)))
	require.NoError(t, err)
	tFormat = TFormat{}
	err = types.AnyBytes(buff).Decode(&tFormat)
	require.NoError(t, err)
	require.Equal(t,
		map[TResourceType]map[TResourceName]TResourceProperties{
			TResourceType("softlayer_file_storage"): {
				TResourceName(name + "-worker_fs"): {"fsp1": "fsv1", "fsp2": "fsv2"},
			},
			TResourceType("softlayer_block_storage"): {
				TResourceName(name + "-worker_bs"): {"bsp1": "bsv1", "bsp2": "bsv2"},
			},
		},
		tFormat.Resource,
	)
}

func TestScanLocalFilesNoFiles(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	vms, err := tf.scanLocalFiles()
	require.NoError(t, err)
	require.Empty(t, vms)
}

func TestScanLocalFilesInvalidFile(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	err := afero.WriteFile(tf.fs, filepath.Join(tf.Dir, "instance-12345.tf.json"), []byte("not-json"), 0644)
	require.NoError(t, err)
	_, err = tf.scanLocalFiles()
	require.Error(t, err)
}

func TestScanLocalFilesNoVms(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	// Create a valid file without a VM type
	m := make(map[TResourceType]map[TResourceName]TResourceProperties)
	tformat := TFormat{Resource: m}
	buff, err := json.Marshal(tformat)
	require.NoError(t, err)
	err = afero.WriteFile(tf.fs, filepath.Join(tf.Dir, "instance-12345.tf.json"), buff, 0644)
	require.NoError(t, err)
	_, err = tf.scanLocalFiles()
	require.Error(t, err)
	require.Equal(t, "not found", err.Error())
}

func TestScanLocalFiles(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	// Create a few valid files, same type
	inst1 := make(map[TResourceType]map[TResourceName]TResourceProperties)
	inst1[VMSoftLayer] = map[TResourceName]TResourceProperties{
		"instance-12": {"key1": "val1"},
	}
	tformat := TFormat{Resource: inst1}
	buff, err := json.MarshalIndent(tformat, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(tf.fs, filepath.Join(tf.Dir, "instance-12.tf.json"), buff, 0644)
	require.NoError(t, err)

	inst2 := make(map[TResourceType]map[TResourceName]TResourceProperties)
	inst2[VMSoftLayer] = map[TResourceName]TResourceProperties{
		"instance-34": {"key2": "val2"},
	}
	tformat = TFormat{Resource: inst2}
	buff, err = json.MarshalIndent(tformat, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(tf.fs, filepath.Join(tf.Dir, "instance-34.tf.json"), buff, 0644)
	require.NoError(t, err)

	// And another type
	inst3 := make(map[TResourceType]map[TResourceName]TResourceProperties)
	inst3[VMAmazon] = map[TResourceName]TResourceProperties{
		"instance-56": {"key3": "val3"},
	}
	tformat = TFormat{Resource: inst3}
	buff, err = json.MarshalIndent(tformat, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(tf.fs, filepath.Join(tf.Dir, "instance-56.tf.json"), buff, 0644)
	require.NoError(t, err)

	// Should get 2 different resource types, 2 VMs for softlayer and 1 for AWS
	vms, err := tf.scanLocalFiles()
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
	props := TResourceProperties{PropHostnamePrefix: "prefix"}
	platformSpecificUpdates(VMSoftLayer, "instance-1234", nil, props)
	require.Equal(t, 1, len(props))
	require.Equal(t, "prefix-1234", props["hostname"])
}

func TestPlatformSpecificUpdatesWithHostnamePrefixWithLogicalID(t *testing.T) {
	logicalID := instance.LogicalID("logical-id")
	props := TResourceProperties{PropHostnamePrefix: "prefix"}
	platformSpecificUpdates(VMSoftLayer, "instance-1234", &logicalID, props)
	require.Equal(t, 1, len(props))
	require.Equal(t, "prefix-logical-id", props["hostname"])
}

func TestPlatformSpecificUpdatesWithNonStringHostnamePrefix(t *testing.T) {
	logicalID := instance.LogicalID("logical-id")
	props := TResourceProperties{PropHostnamePrefix: 1, "hostname": "hostname"}
	platformSpecificUpdates(VMSoftLayer, "instance-1234", &logicalID, props)
	require.Equal(t, 1, len(props))
	require.Equal(t, "logical-id", props["hostname"])
}

func TestPlatformSpecificUpdatesWithEmptyHostanmePrefix(t *testing.T) {
	props := TResourceProperties{PropHostnamePrefix: "", "hostname": "hostname"}
	platformSpecificUpdates(VMSoftLayer, "instance-1234", nil, props)
	require.Equal(t, 1, len(props))
	require.Equal(t, "instance-1234", props["hostname"])
}

func TestMergeTagsIntoVMPropsEmpty(t *testing.T) {
	for _, vmType := range VMTypes {
		props := TResourceProperties{}
		mergeTagsIntoVMProps(vmType.(TResourceType), props, map[string]string{})
		var expectedTags interface{}
		if vmType == VMSoftLayer || vmType == VMIBMCloud {
			expectedTags = []interface{}{}
		} else {
			expectedTags = map[string]interface{}{}
		}
		require.Equal(t, expectedTags, props["tags"])
	}
}

func TestMergeTagsIntoVMPropsNoExtraTags(t *testing.T) {
	for _, vmType := range VMTypes {
		var props TResourceProperties
		if vmType == VMSoftLayer || vmType == VMIBMCloud {
			props = TResourceProperties{
				"tags": []interface{}{
					"Name:instance-1234",
					"foo:BaR",
				},
			}
		} else {
			props = TResourceProperties{
				"tags": map[string]interface{}{
					"Name": "instance-1234",
					"foo":  "BaR",
				},
			}
		}
		mergeTagsIntoVMProps(vmType.(TResourceType), props, map[string]string{})
		if vmType == VMSoftLayer || vmType == VMIBMCloud {
			tags := props["tags"]
			require.Len(t, tags, 2)
			// Note that tags are all lowercase
			require.Contains(t, tags, "foo:bar")
			require.Contains(t, tags, "name:instance-1234")
		} else {
			expectedTags := map[string]interface{}{
				"Name": "instance-1234",
				"foo":  "BaR",
			}
			require.Equal(t, expectedTags, props["tags"])
		}

	}
}

func TestMergeTagsIntoVMPropsNoVMTags(t *testing.T) {
	for _, vmType := range VMTypes {
		tags := map[string]string{
			"Name": "instance-1234",
			"foo":  "BaR",
		}
		props := TResourceProperties{}
		mergeTagsIntoVMProps(vmType.(TResourceType), props, tags)
		if vmType == VMSoftLayer || vmType == VMIBMCloud {
			tags := props["tags"]
			require.Len(t, tags, 2)
			// Note that tags are all lowercase
			require.Contains(t, tags, "foo:bar")
			require.Contains(t, tags, "name:instance-1234")
		} else {
			expectedTags := map[string]interface{}{
				"Name": "instance-1234",
				"foo":  "BaR",
			}
			require.Equal(t, expectedTags, props["tags"])
		}
	}
}

func TestMergeTagsIntoVMProps(t *testing.T) {
	for _, vmType := range VMTypes {
		var props TResourceProperties
		if vmType == VMSoftLayer || vmType == VMIBMCloud {
			props = TResourceProperties{
				"tags": []interface{}{
					"Name:instance-1234",
					"key:original",
				},
			}
		} else {
			props = TResourceProperties{
				"tags": map[string]interface{}{
					"Name": "instance-1234",
					"key":  "original",
				},
			}
		}
		tags := map[string]string{
			"Name": "instance-1234",
			"key":  "override::val",
			// Input tag is comma separated
			attachTag: fmt.Sprintf("%s,%s", "attach1", "attach2"),
		}
		mergeTagsIntoVMProps(vmType.(TResourceType), props, tags)
		if vmType == VMSoftLayer || vmType == VMIBMCloud {
			tags := props["tags"]
			require.Len(t, tags, 3)
			require.Contains(t, tags, "key:override::val")
			require.Contains(t, tags, "name:instance-1234")
			// Changed to space separated
			require.Contains(t,
				tags,
				fmt.Sprintf("%s:%s %s", attachTag, "attach1", "attach2"),
			)
		} else {
			expectedTags := map[string]interface{}{
				"Name":    "instance-1234",
				"key":     "override::val",
				attachTag: fmt.Sprintf("%s,%s", "attach1", "attach2"),
			}
			require.Equal(t, expectedTags, props["tags"])
		}
	}
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
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	err := tf.Label(instance.ID("ID"), nil)
	require.Error(t, err)
}

func TestLabelInvalidFile(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	id := "instance-1234"
	err := afero.WriteFile(tf.fs, filepath.Join(tf.Dir, fmt.Sprintf("%v.tf.json", id)), []byte("not-json"), 0644)
	require.NoError(t, err)
	err = tf.Label(instance.ID(id), nil)
	require.Error(t, err)
}

func TestLabelNoVM(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	id := "instance-1234"
	// No VM data in instance definition
	inst := make(map[TResourceType]map[TResourceName]TResourceProperties)
	tformat := TFormat{Resource: inst}
	buff, err := json.MarshalIndent(tformat, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(tf.fs, filepath.Join(tf.Dir, fmt.Sprintf("%v.tf.json", id)), buff, 0644)
	require.NoError(t, err)
	err = tf.Label(instance.ID(id), nil)
	require.Error(t, err)
	require.Equal(t, "not found", err.Error())
}

func TestLabelNoProperties(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	id := "instance-1234"
	// Resource does not have any properties
	inst := make(map[TResourceType]map[TResourceName]TResourceProperties)
	inst[VMSoftLayer] = map[TResourceName]TResourceProperties{"instance-1234": {}}
	tformat := TFormat{Resource: inst}
	buff, err := json.MarshalIndent(tformat, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(tf.fs, filepath.Join(tf.Dir, fmt.Sprintf("%v.tf.json", id)), buff, 0644)
	require.NoError(t, err)
	err = tf.Label(instance.ID(id), nil)
	require.Error(t, err)
	require.Equal(t, "not found:instance-1234", err.Error())
}

func TestLabelCreateNewTags(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

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
		err = afero.WriteFile(tf.fs, filepath.Join(tf.Dir, fmt.Sprintf("%v.tf.json", id)), buff, 0644)
		require.NoError(t, err)
	}

	// Add some labels
	labels := map[string]string{
		"label1": "value1",
		"label2": "value2",
	}
	for index := range VMTypes {
		id := fmt.Sprintf("instance-%v", index)
		err := tf.Label(instance.ID(id), labels)
		require.NoError(t, err)
	}

	// Verify that the labels were added
	for index, vmType := range VMTypes {
		id := fmt.Sprintf("instance-%v", index)
		buff, err := ioutil.ReadFile(filepath.Join(tf.Dir, id+".tf.json"))
		require.NoError(t, err)
		tFormat := TFormat{}
		err = types.AnyBytes(buff).Decode(&tFormat)
		require.NoError(t, err)
		actualVMType, vmName, props, err := FindVM(&tFormat)
		require.NoError(t, err)
		require.Equal(t, vmType, actualVMType)
		require.Equal(t, TResourceName(id), vmName)
		_, contains := props["tags"]
		require.True(t, contains)
		if vmType == VMSoftLayer || vmType == VMIBMCloud {
			// Tags as list
			tags := props["tags"]
			require.Len(t, tags, 2)
			require.Contains(t, tags, "label1:value1")
			require.Contains(t, tags, "label2:value2")
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
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	// Create a file with existing tags for each VMType
	for index, vmType := range VMTypes {
		inst := make(map[TResourceType]map[TResourceName]TResourceProperties)
		id := fmt.Sprintf("instance-%v", index)
		key := vmType.(TResourceType)
		var tags interface{}
		if vmType == VMSoftLayer || vmType == VMIBMCloud {
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
		err = afero.WriteFile(tf.fs, filepath.Join(tf.Dir, fmt.Sprintf("%v.tf.json", id)), buff, 0644)
		require.NoError(t, err)
	}

	// Add some labels
	labels := map[string]string{
		"label1": "value1",
		"label2": "value2",
	}
	for index := range VMTypes {
		id := fmt.Sprintf("instance-%v", index)
		err := tf.Label(instance.ID(id), labels)
		require.NoError(t, err)
	}

	// Verify that the labels were added
	for index, vmType := range VMTypes {
		id := fmt.Sprintf("instance-%v", index)
		buff, err := ioutil.ReadFile(filepath.Join(tf.Dir, id+".tf.json"))
		require.NoError(t, err)
		tFormat := TFormat{}
		err = types.AnyBytes(buff).Decode(&tFormat)
		require.NoError(t, err)
		actualVMType, vmName, props, err := FindVM(&tFormat)
		require.NoError(t, err)
		require.Equal(t, vmType, actualVMType)
		require.Equal(t, TResourceName(id), vmName)
		_, contains := props["tags"]
		require.True(t, contains)
		if vmType == VMSoftLayer || vmType == VMIBMCloud {
			// Tags as list
			tags := props["tags"]
			require.Len(t, tags, 4)
			require.Contains(t, tags, "tag1:val1")
			require.Contains(t, tags, "tag2:val2")
			require.Contains(t, tags, "label1:value1")
			require.Contains(t, tags, "label2:value2")
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

func TestParseTerraformTagsEmpty(t *testing.T) {
	// No tags
	props := TResourceProperties{"foo": "bar"}
	for _, vmType := range VMTypes {
		result := parseTerraformTags(vmType.(TResourceType), props)
		require.Equal(t, map[string]string{}, result)
	}
	// Invalid type
	props = TResourceProperties{
		"foo":  "bar",
		"tags": true,
	}
	for _, vmType := range VMTypes {
		result := parseTerraformTags(vmType.(TResourceType), props)
		require.Equal(t, map[string]string{}, result)
	}
}

func TestParseTerraformTags(t *testing.T) {
	for _, vmType := range VMTypes {
		var props TResourceProperties
		switch vmType {
		case VMAmazon, VMAzure, VMDigitalOcean, VMGoogleCloud:
			props = TResourceProperties{
				"foo": "bar",
				"tags": map[string]interface{}{
					"t1": "v1",
					"t2": "v2",
					"t3": "v3:extra",
				},
			}
		case VMSoftLayer, VMIBMCloud:
			props = TResourceProperties{
				"foo": "bar",
				"tags": []interface{}{
					"t1:v1",
					"t2:v2",
					"t3:v3:extra",
				},
			}
		default:
			require.Fail(t, fmt.Sprintf("parseTerraformTags not handled for type: %v", vmType))
		}
		result := parseTerraformTags(vmType.(TResourceType), props)
		require.Equal(t,
			map[string]string{"t1": "v1", "t2": "v2", "t3": "v3:extra"},
			result,
		)
	}
}

func TestParseTerraformTagsInfrakitAttach(t *testing.T) {
	for _, vmType := range VMTypes {
		var props TResourceProperties
		switch vmType {
		case VMAmazon, VMAzure, VMDigitalOcean, VMGoogleCloud:
			props = TResourceProperties{
				"foo": "bar",
				"tags": map[string]interface{}{
					"infrakit.attach": "attach1,attach2",
				},
			}
		case VMSoftLayer, VMIBMCloud:
			props = TResourceProperties{
				"foo": "bar",
				"tags": []interface{}{
					// Space should be parsed to a comma
					"infrakit.attach:attach1 attach2",
				},
			}
		default:
			require.Fail(t, fmt.Sprintf("parseTerraformTags not handled for type: %v", vmType))
		}
		result := parseTerraformTags(vmType.(TResourceType), props)
		require.Equal(t,
			map[string]string{"infrakit.attach": "attach1,attach2"},
			result,
		)
	}
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
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	err := tf.Destroy(instance.ID("id"), instance.Termination)
	require.Error(t, err)
}

func TestDestroy(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	id := "instance-1234"
	tformat := TFormat{
		Resource: map[TResourceType]map[TResourceName]TResourceProperties{
			VMSoftLayer: {
				TResourceName("host"): {},
			},
		},
	}
	buff, err := json.MarshalIndent(tformat, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(tf.fs, filepath.Join(tf.Dir, fmt.Sprintf("%v.tf.json", id)), buff, 0644)
	require.NoError(t, err)
	err = tf.Destroy(instance.ID(id), instance.Termination)
	require.Nil(t, err)

	// The file has been removed
	files, err := ioutil.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 0)
}

func TestDestroyScaleDown(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	m := map[TResourceType]map[TResourceName]TResourceProperties{
		VMSoftLayer: {
			TResourceName("host"): {},
		},
		TResourceType("softlayer_file_storage"): {
			TResourceName("worker_fs"): {
				PropScope: ValScopeDedicated,
			},
		},
	}
	tformat := TFormat{Resource: m}
	buff, err := json.MarshalIndent(tformat, "  ", "  ")
	require.NoError(t, err)
	id, err := tf.Provision(instance.Spec{
		Properties: types.AnyBytes(buff),
		Tags:       map[string]string{"tag1": "val1"},
	})
	require.NoError(t, err)
	// 2 files created
	files, err := ioutil.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 2)
	// Destroy the instance and the related files
	err = tf.Destroy(instance.ID(*id), instance.Termination)
	require.NoError(t, err)
	// All files has been removed
	files, err = ioutil.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 0)
}

func TestDestroyRollingUpdate(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	m := map[TResourceType]map[TResourceName]TResourceProperties{
		VMSoftLayer: {
			TResourceName("host"): {},
		},
		TResourceType("softlayer_file_storage"): {
			TResourceName("worker_fs"): {
				PropScope: ValScopeDedicated,
			},
		},
	}
	tformat := TFormat{Resource: m}
	buff, err := json.MarshalIndent(tformat, "  ", "  ")
	require.NoError(t, err)
	id, err := tf.Provision(instance.Spec{
		Properties: types.AnyBytes(buff),
		Tags:       map[string]string{"tag1": "val1"},
	})
	require.NoError(t, err)
	// 2 files created
	files, err := ioutil.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 2)
	// Destroy the instance and the related files
	// TODO(kaufers): Update to SPI call once rolling update context is supported
	err = tf.doDestroy(instance.ID(*id), false)
	require.NoError(t, err)
	// Instance file has been removed; dedicated file still exists
	files, err = ioutil.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	path := filepath.Join(dir, fmt.Sprintf("%s-dedicated.tf.json", string(*id)))
	_, err = ioutil.ReadFile(path)
	require.NoError(t, err, fmt.Sprintf("Expected path %s does not exist", path))
}

func TestParseAttachTagFromFileNoFile(t *testing.T) {
	_, err := parseAttachTagFromFile("")
	require.Error(t, err)
}

func TestParseAttachTagFromFileNoVM(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	tformat := TFormat{
		Resource: map[TResourceType]map[TResourceName]TResourceProperties{},
	}
	buff, err := json.MarshalIndent(tformat, " ", " ")
	require.NoError(t, err)
	fp := filepath.Join(tf.Dir, "instance-1234.tf.json")
	err = afero.WriteFile(tf.fs, fp, buff, 0644)
	require.NoError(t, err)
	_, err = parseAttachTagFromFile(fp)
	require.Error(t, err)
	require.Equal(t, "not found", err.Error())
}

func TestParseAttachTagFromFileMap(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	tformat := TFormat{
		Resource: map[TResourceType]map[TResourceName]TResourceProperties{
			VMAmazon: {
				TResourceName("host1"): {
					"tags": map[string]interface{}{
						"foo":     "bar",
						attachTag: "attach1,attach2",
					},
				},
			},
		},
	}
	buff, err := json.MarshalIndent(tformat, " ", " ")
	require.NoError(t, err)
	fp := filepath.Join(tf.Dir, "instance-1234.tf.json")
	err = afero.WriteFile(tf.fs, fp, buff, 0644)
	require.NoError(t, err)
	results, err := parseAttachTagFromFile(fp)
	require.NoError(t, err)
	require.Equal(t, []string{"attach1", "attach2"}, results)
}

func TestParseAttachTagFromFileSlice(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	tformat := TFormat{
		Resource: map[TResourceType]map[TResourceName]TResourceProperties{
			VMSoftLayer: {
				TResourceName("host1"): {
					"tags": []interface{}{
						"foo:bar",
						fmt.Sprintf("%s:attach1 attach2", attachTag),
					},
				},
			},
		},
	}
	buff, err := json.MarshalIndent(tformat, " ", " ")
	require.NoError(t, err)
	fp := filepath.Join(tf.Dir, "instance-1234.tf.json")
	err = afero.WriteFile(tf.fs, fp, buff, 0644)
	require.NoError(t, err)
	results, err := parseAttachTagFromFile(fp)
	require.NoError(t, err)
	require.Equal(t, []string{"attach1", "attach2"}, results)
}

func TestDescribeNoFiles(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	results, err := tf.DescribeInstances(map[string]string{}, false)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{}, results)
}

func TestDescribe(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	// Instance1, unique tag and shared tag
	inst1 := make(map[TResourceType]map[TResourceName]TResourceProperties)
	id1 := "instance-1"
	tags1 := []string{"tag1:val1", "tagShared:valShared"}
	inst1[VMSoftLayer] = map[TResourceName]TResourceProperties{
		TResourceName(id1): {"tags": tags1},
	}
	buff, err := json.MarshalIndent(TFormat{Resource: inst1}, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(tf.fs, filepath.Join(tf.Dir, fmt.Sprintf("%v.tf.json", id1)), buff, 0644)
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
	err = afero.WriteFile(tf.fs, filepath.Join(tf.Dir, fmt.Sprintf("%v.tf.json", id2)), buff, 0644)
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
	err = afero.WriteFile(tf.fs, filepath.Join(tf.Dir, fmt.Sprintf("%v.tf.json", id3)), buff, 0644)
	require.NoError(t, err)

	// First instance matches
	results, err := tf.DescribeInstances(
		map[string]string{"tag1": "val1"},
		false)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, instance.ID(id1), results[0].ID)
	results, err = tf.DescribeInstances(
		map[string]string{"tag1": "val1", "tagShared": "valShared"},
		false)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, instance.ID(id1), results[0].ID)

	// Second instance matches
	results, err = tf.DescribeInstances(
		map[string]string{"tag2": "val2"},
		false)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, instance.ID(id2), results[0].ID)

	// Both instances matches
	results, err = tf.DescribeInstances(
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
	results, err = tf.DescribeInstances(
		map[string]string{"tag1": "val1", "tagShared": "valShared", "foo": "bar"},
		false)
	require.NoError(t, err)
	require.Empty(t, results)
	results, err = tf.DescribeInstances(
		map[string]string{"TAG2": "val2"},
		false)
	require.NoError(t, err)
	require.Empty(t, results)

	// All instances match (no tags passed)
	results, err = tf.DescribeInstances(map[string]string{}, false)
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

func TestDescribeAttachTag(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	inst1 := make(map[TResourceType]map[TResourceName]TResourceProperties)
	id1 := "instance-1"
	inst1[VMSoftLayer] = map[TResourceName]TResourceProperties{
		TResourceName(id1): {
			"tags": []string{
				"key:val",
				// Tag value has space delimiter
				fmt.Sprintf("%s:attach1-1 attach1-2", attachTag),
			},
		},
	}
	buff, err := json.MarshalIndent(TFormat{Resource: inst1}, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(tf.fs, filepath.Join(tf.Dir, fmt.Sprintf("%v.tf.json", id1)), buff, 0644)
	require.NoError(t, err)

	inst2 := make(map[TResourceType]map[TResourceName]TResourceProperties)
	id2 := "instance-2"
	inst2[VMAzure] = map[TResourceName]TResourceProperties{
		TResourceName(id2): {
			"tags": map[string]string{
				"key": "val",
				// Tag value has comma delimiter
				attachTag: "attach2-1,attach2-2",
			},
		},
	}
	buff, err = json.MarshalIndent(TFormat{Resource: inst2}, " ", " ")
	require.NoError(t, err)
	err = afero.WriteFile(tf.fs, filepath.Join(tf.Dir, fmt.Sprintf("%v.tf.json", id2)), buff, 0644)
	require.NoError(t, err)

	// Get both instances
	results, err := tf.DescribeInstances(
		map[string]string{"key": "val"},
		false)
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Contains(t,
		results,
		instance.Description{
			ID: instance.ID(id1),
			Tags: map[string]string{
				"key":     "val",
				attachTag: "attach1-1,attach1-2",
			},
		},
	)
	require.Contains(t,
		results,
		instance.Description{
			ID: instance.ID(id2),
			Tags: map[string]string{
				"key":     "val",
				attachTag: "attach2-1,attach2-2",
			},
		},
	)
}

func TestMergePropNotInSource(t *testing.T) {
	source := TResourceProperties{}
	dest := TResourceProperties{"key": "val"}
	mergeProp(source, dest, "foo")
	require.Equal(t, TResourceProperties{"key": "val"}, dest)
}

func TestMergePropNotInDest(t *testing.T) {
	source := TResourceProperties{"key": "val"}
	dest := TResourceProperties{}
	mergeProp(source, dest, "key")
	require.Equal(t, TResourceProperties{"key": "val"}, dest)
}

func TestMergeNonComplex(t *testing.T) {
	source := TResourceProperties{"key": "new-val", "other": "z"}
	dest := TResourceProperties{"key": "old-val", "foo": "bar"}
	mergeProp(source, dest, "key")
	require.Equal(t,
		TResourceProperties{"key": "new-val", "foo": "bar"},
		dest)
}

func TestMergePropSliceIntoEmptySlice(t *testing.T) {
	source := TResourceProperties{"key": []interface{}{1, 2, true}}
	dest := TResourceProperties{
		"key": []interface{}{},
		"foo": "bar",
	}
	mergeProp(source, dest, "key")
	require.Equal(t,
		TResourceProperties{
			"key": []interface{}{1, 2, true},
			"foo": "bar",
		},
		dest)
}

func TestMergePropSliceIntoSlice(t *testing.T) {
	source := TResourceProperties{"key": []interface{}{1, 2, true}}
	dest := TResourceProperties{
		"key": []interface{}{1},
		"foo": "bar",
	}
	mergeProp(source, dest, "key")
	require.Equal(t,
		TResourceProperties{
			"key": []interface{}{1, 2, true},
			"foo": "bar",
		},
		dest)
}

func TestMergeTagsSliceIntoEmptySlice(t *testing.T) {
	source := TResourceProperties{"tags": []interface{}{"tag1:val1", "tag2:val2"}}
	dest := TResourceProperties{
		"tags": []interface{}{},
		"foo":  "bar",
	}
	mergeProp(source, dest, "tags")
	require.Equal(t,
		TResourceProperties{
			"tags": []interface{}{"tag1:val1", "tag2:val2"},
			"foo":  "bar",
		},
		dest)
}

func TestMergeTagsSliceIntoSlice(t *testing.T) {
	source := TResourceProperties{"tags": []interface{}{"tag1:val1", "tag2:override"}}
	dest := TResourceProperties{
		"tags": []interface{}{"tag2:val2", "tag3:val3"},
		"foo":  "bar",
	}
	mergeProp(source, dest, "tags")
	require.Equal(t,
		TResourceProperties{
			"tags": []interface{}{"tag2:override", "tag3:val3", "tag1:val1"},
			"foo":  "bar",
		},
		dest)
}

func TestMergeSliceIntoWrongType(t *testing.T) {
	source := TResourceProperties{"slice": []interface{}{1, 2, 3}}
	dest := TResourceProperties{
		"slice": true,
		"foo":   "bar",
	}
	mergeProp(source, dest, "slice")
	require.Equal(t,
		TResourceProperties{
			"slice": []interface{}{1, 2, 3},
			"foo":   "bar",
		},
		dest)
}

func TestMergePropMapIntoEmptyMap(t *testing.T) {
	source := TResourceProperties{
		"key": map[string]interface{}{
			"k1": "v1",
			"k2": "v2",
		},
	}
	dest := TResourceProperties{
		"key": map[string]interface{}{},
		"foo": "bar",
	}
	mergeProp(source, dest, "key")
	require.Equal(t,
		TResourceProperties{
			"key": map[string]interface{}{
				"k1": "v1",
				"k2": "v2",
			},
			"foo": "bar",
		},
		dest)
}

func TestMergePropMapIntoMap(t *testing.T) {
	source := TResourceProperties{
		"key": map[string]interface{}{
			"k1": "v1",
			"k2": "v-override",
		},
	}
	dest := TResourceProperties{
		"key": map[string]interface{}{
			"k1": "v1",
			"k2": "v2",
			"k3": "v3",
		},
		"foo": "bar",
	}
	mergeProp(source, dest, "key")
	require.Equal(t,
		TResourceProperties{
			"key": map[string]interface{}{
				"k1": "v1",
				"k2": "v-override",
				"k3": "v3",
			},
			"foo": "bar",
		},
		dest)
}

func TestMergeMapIntoWrongType(t *testing.T) {
	source := TResourceProperties{
		"map": map[string]interface{}{
			"k1": "v1",
			"k2": "v2",
		},
	}
	dest := TResourceProperties{
		"map": true,
		"foo": "bar",
	}
	mergeProp(source, dest, "map")
	require.Equal(t,
		TResourceProperties{
			"map": map[string]interface{}{
				"k1": "v1",
				"k2": "v2",
			},
			"foo": "bar",
		},
		dest)
}

func TestWriteTfJSONForImport(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	specProps := TResourceProperties{
		PropHostnamePrefix: "some-prefix",
		PropScope:          "some-scope",
		"ssh-key-ids":      []interface{}{789},
		"datacenter":       "some-datacenter",
		"tags":             []interface{}{"spec-tag1:spec-val1"},
		"z-other":          "not-imported",
	}
	importedProps := TResourceProperties{
		"hostname":    "actual-hostname",
		"ssh-key-ids": []interface{}{123},
		"datacenter":  "actual-datacenter",
		"tags": []interface{}{
			"actual-tag1:actual-val1",
			"actual-tag2:actual-val2",
		},
		"ip":         "10.0.0.1",
		"z-imported": "imported-but-not-in-spec",
	}

	id := "instance-12345"
	err := tf.writeTfJSONForImport(specProps, importedProps, VMIBMCloud, id)
	require.NoError(t, err)
	buff, err := ioutil.ReadFile(filepath.Join(tf.Dir, id+".tf.json"))
	require.NoError(t, err)
	tFormat := TFormat{}
	err = types.AnyBytes(buff).Decode(&tFormat)
	require.NoError(t, err)
	actualVMType, vmName, props, err := FindVM(&tFormat)
	require.NoError(t, err)
	require.Equal(t, VMIBMCloud, actualVMType)
	require.Equal(t, TResourceName("instance-12345"), vmName)
	expectedProps := TResourceProperties{
		"hostname":    "actual-hostname",
		"ssh-key-ids": []interface{}{float64(123)},
		"datacenter":  "actual-datacenter",
		"tags": []interface{}{
			"spec-tag1:spec-val1",
			"actual-tag1:actual-val1",
			"actual-tag2:actual-val2",
		},
	}
	require.Equal(t, expectedProps, props)
}

func TestImportInvalidGrpSpecProps(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	spec := group.Spec{
		ID:         "managers",
		Properties: types.AnyString("no-json"),
	}
	_, err := tf.importResource(importFns{}, "123", spec, true)
	require.Error(t, err)
}

func TestImportNoVm(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	spec := group.Spec{
		ID:         "managers",
		Properties: types.AnyString("{}"),
	}
	_, err := tf.importResource(importFns{}, "123", spec, true)
	require.Error(t, err)
	require.Equal(t, "no resource section", err.Error())
}

func TestImportNoVmProps(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	spec := group.Spec{
		ID: "managers",
		Properties: types.AnyString(`
{
  "instance": {
    "Properties": {
      "resource": {
        "aws_instance": {}
      }
    }
  }
}`)}
	_, err := tf.importResource(importFns{}, "123", spec, true)
	require.Error(t, err)
	require.Equal(t, "Missing resource properties", err.Error())
}

func TestImportTfShowError(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	fns := importFns{
		tfShow: func(dirArg string, vmType TResourceType) (map[TResourceName]TResourceProperties, error) {
			require.Equal(t, dir, dirArg)
			require.Equal(t, VMAmazon, vmType)
			return nil, fmt.Errorf("Custom show error")
		},
	}
	spec := group.Spec{
		ID: "managers",
		Properties: types.AnyString(`
{
  "instance": {
    "Properties": {
      "resource": {
        "aws_instance": {
          "host": {
            "hostnane": "host1"
          }
        }
      }
    }
  }
}`)}
	_, err := tf.importResource(fns, "123", spec, true)
	require.Error(t, err)
	require.Equal(t, "Custom show error", err.Error())
}

func TestImportAlreadyExists(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	fns := importFns{
		tfShow: func(dirArg string, vmType TResourceType) (map[TResourceName]TResourceProperties, error) {
			require.Equal(t, dir, dirArg)
			require.Equal(t, VMAmazon, vmType)
			return map[TResourceName]TResourceProperties{
				TResourceName("instance-foo"): {},
				TResourceName("instance-123"): {
					"id": 123,
				},
			}, nil
		},
	}
	spec := group.Spec{
		ID: "managers",
		Properties: types.AnyString(`
{
  "instance": {
    "Properties": {
      "resource": {
        "aws_instance": {
          "host": {
            "hostnane": "host1"
          }
        }
      }
    }
  }
}`)}
	id, err := tf.importResource(fns, "123", spec, true)
	require.NoError(t, err)
	require.Equal(t, "instance-123", string(*id))
}

func TestImportTfImportError(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	cleanInvoked := false
	fns := importFns{
		tfShow: func(dirArg string, vmType TResourceType) (map[TResourceName]TResourceProperties, error) {
			require.Equal(t, dir, dirArg)
			require.Equal(t, VMAmazon, vmType)
			return map[TResourceName]TResourceProperties{}, nil
		},
		tfImport: func(vmType TResourceType, filename, vmID string) error {
			require.Equal(t, VMAmazon, vmType)
			require.True(t, strings.HasPrefix(filename, "instance-"))
			require.Equal(t, "123", vmID)
			return fmt.Errorf("Custom import error")
		},
		tfClean: func(vmType TResourceType, vmName string) {
			require.Equal(t, VMAmazon, vmType)
			require.True(t, strings.HasPrefix(vmName, "instance-"))
			cleanInvoked = true
		},
	}
	spec := group.Spec{
		ID: "managers",
		Properties: types.AnyString(`
{
  "instance": {
    "Properties": {
      "resource": {
        "aws_instance": {
          "host": {
            "hostnane": "host1"
          }
        }
      }
    }
  }
}`)}
	_, err := tf.importResource(fns, "123", spec, true)
	require.Error(t, err)
	require.Equal(t, "Custom import error", err.Error())
	require.True(t, cleanInvoked)
}

func TestImportTfShowInstError(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	cleanInvoked := false
	fns := importFns{
		tfShow: func(dirArg string, vmType TResourceType) (map[TResourceName]TResourceProperties, error) {
			require.Equal(t, dir, dirArg)
			require.Equal(t, VMAmazon, vmType)
			return map[TResourceName]TResourceProperties{}, nil
		},
		tfImport: func(vmType TResourceType, filename, vmID string) error {
			require.Equal(t, VMAmazon, vmType)
			require.True(t, strings.HasPrefix(filename, "instance-"))
			require.Equal(t, "123", vmID)
			return nil
		},
		tfShowInst: func(dirArg, id string) (TResourceProperties, error) {
			require.Equal(t, dir, dirArg)
			return nil, fmt.Errorf("Custom show inst error")
		},
		tfClean: func(vmType TResourceType, vmName string) {
			require.Equal(t, VMAmazon, vmType)
			require.True(t, strings.HasPrefix(vmName, "instance-"))
			cleanInvoked = true
		},
	}
	spec := group.Spec{
		ID: "managers",
		Properties: types.AnyString(`
{
  "instance": {
    "Properties": {
      "resource": {
        "aws_instance": {
          "host": {
            "hostnane": "host1"
          }
        }
      }
    }
  }
}`)}
	_, err := tf.importResource(fns, "123", spec, true)
	require.Error(t, err)
	require.Equal(t, "Custom show inst error", err.Error())
	require.True(t, cleanInvoked)
}

func TestImportResourceTagMap(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	cleanInvoked := false
	fns := importFns{
		tfShow: func(dirArg string, vmType TResourceType) (map[TResourceName]TResourceProperties, error) {
			require.Equal(t, dir, dirArg)
			require.Equal(t, VMAmazon, vmType)
			return map[TResourceName]TResourceProperties{}, nil
		},
		tfImport: func(vmType TResourceType, filename, vmID string) error {
			require.Equal(t, VMAmazon, vmType)
			require.True(t, strings.HasPrefix(filename, "instance-"))
			require.Equal(t, "123", vmID)
			return nil
		},
		tfShowInst: func(dirArg, id string) (TResourceProperties, error) {
			require.Equal(t, dir, dirArg)
			require.True(t, strings.HasPrefix(id, "aws_instance.instance-"))
			props := TResourceProperties{
				"hostname": "actual-hostname",
				"spec-key": "actual-val",
				"other":    "foo",
			}
			return props, nil
		},
		tfClean: func(vmType TResourceType, vmName string) {
			cleanInvoked = true
		},
	}
	spec := group.Spec{
		ID: "managers",
		Properties: types.AnyString(`
{
  "instance": {
    "Properties": {
      "resource": {
        "aws_instance": {
          "host": {
            "@hostname_prefix": "host1",
            "spec-key": "spec-val",
            "tags": {"t1": "v1"}
          }
        }
      }
    }
  }
}`)}
	id, err := tf.importResource(fns, "123", spec, true)
	require.NoError(t, err)
	require.False(t, cleanInvoked)

	buff, err := ioutil.ReadFile(filepath.Join(tf.Dir, fmt.Sprintf("%v.tf.json", *id)))
	require.NoError(t, err)
	tFormat := TFormat{}
	err = types.AnyBytes(buff).Decode(&tFormat)
	require.NoError(t, err)
	actualVMType, vmName, props, err := FindVM(&tFormat)
	require.NoError(t, err)
	require.Equal(t, VMAmazon, actualVMType)
	require.Equal(t, string(*id), string(vmName))
	require.Equal(t,
		TResourceProperties{
			"hostname": "actual-hostname",
			"spec-key": "actual-val",
			"tags": map[string]interface{}{
				"t1":                  "v1",
				"infrakit.group":      "managers",
				"infrakit.config_sha": "bootstrap",
			},
		},
		props)
}

func TestImportResourceTagSlice(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	cleanInvoked := false
	fns := importFns{
		tfShow: func(dirArg string, vmType TResourceType) (map[TResourceName]TResourceProperties, error) {
			require.Equal(t, dir, dirArg)
			require.Equal(t, VMIBMCloud, vmType)
			return map[TResourceName]TResourceProperties{}, nil
		},
		tfImport: func(vmType TResourceType, filename, vmID string) error {
			require.Equal(t, VMIBMCloud, vmType)
			require.True(t, strings.HasPrefix(filename, "instance-"))
			require.Equal(t, "123", vmID)
			return nil
		},
		tfShowInst: func(dirArg, id string) (TResourceProperties, error) {
			require.Equal(t, dir, dirArg)
			require.True(t, strings.HasPrefix(id, "ibm_compute_vm_instance.instance-"))
			props := TResourceProperties{
				"hostname": "actual-hostname",
				"spec-key": "actual-val",
				"other":    "foo",
			}
			return props, nil
		},
		tfClean: func(vmType TResourceType, vmName string) {
			cleanInvoked = true
		},
	}
	spec := group.Spec{
		ID: "managers",
		Properties: types.AnyString(`
{
  "instance": {
    "Properties": {
      "resource": {
        "ibm_compute_vm_instance": {
          "host": {
            "@hostname_prefix": "host1",
            "spec-key": "spec-val"
          }
        }
      }
    }
  }
}`)}
	id, err := tf.importResource(fns, "123", spec, true)
	require.NoError(t, err)
	require.False(t, cleanInvoked)

	buff, err := ioutil.ReadFile(filepath.Join(tf.Dir, fmt.Sprintf("%v.tf.json", *id)))
	require.NoError(t, err)
	tFormat := TFormat{}
	err = types.AnyBytes(buff).Decode(&tFormat)
	require.NoError(t, err)
	actualVMType, vmName, props, err := FindVM(&tFormat)
	require.NoError(t, err)
	require.Equal(t, VMIBMCloud, actualVMType)
	require.Equal(t, string(*id), string(vmName))
	// Tag slice order not guaranteed since it is created by iterating over a map
	tags := props["tags"]
	delete(props, "tags")
	require.Len(t, tags, 2)
	require.Contains(t, tags, "infrakit.group:managers")
	require.Contains(t, tags, "infrakit.config_sha:bootstrap")
	// Compare everythine else
	require.Equal(t,
		TResourceProperties{
			"hostname": "actual-hostname",
			"spec-key": "actual-val",
		},
		props)
}

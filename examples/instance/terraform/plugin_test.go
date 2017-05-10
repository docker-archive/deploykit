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
	"github.com/stretchr/testify/require"
)

func TestUsage(t *testing.T) {
	// Test a softlayer_virtual_guest with an @hostname_prefix
	run(t, "softlayer_virtual_guest", `
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
	run(t, "softlayer_virtual_guest", `
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
	run(t, "softlayer_virtual_guest", `
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

	run(t, "aws_instance", `
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

func firstInMap(m map[string]interface{}) (string, interface{}) {
	for k, v := range m {
		return k, v
	}
	return "", nil
}

func run(t *testing.T, resourceType, properties string) {
	dir, err := ioutil.TempDir("", "infrakit-instance-terraform")
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	terraform := NewTerraformInstancePlugin(dir)
	terraform.(*plugin).pretend = true // turn off actually calling terraform

	config := types.AnyString(properties)

	err = terraform.Validate(config)
	require.NoError(t, err)

	// Instance with tags that will not be updated
	instanceSpec1 := instance.Spec{
		Properties: config,
		Tags: map[string]string{
			"label1": "value1",
			"label2": "value2",
		},
		Init:        "",
		Attachments: []instance.Attachment{},
	}
	id1, err := terraform.Provision(instanceSpec1)
	require.NoError(t, err)
	tfPath := filepath.Join(dir, string(*id1)+".tf.json")
	_, err = ioutil.ReadFile(tfPath)
	require.NoError(t, err)

	// Instance with tags that will be updated
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
	}
	id2, err := terraform.Provision(instanceSpec2)
	require.NoError(t, err)
	require.NotEqual(t, id1, id2)

	tfPath = filepath.Join(dir, string(*id2)+".tf.json")
	buff, err := ioutil.ReadFile(tfPath)
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
			}), conv(props["tags"].([]interface{})))
			require.Equal(t, expectedUserData2, props["user_metadata"])

			// If a hostname was specified, the expectation is that the hostname is appended with the timestamp from the ID
			if value["@hostname_prefix"] != nil && strings.Trim(value["@hostname_prefix"].(string), " ") != "" {
				newID := strings.Replace(string(*id2), "instance-", "", -1)
				expectedHostname := "softlayer-hostname-" + newID
				require.Equal(t, expectedHostname, props["hostname"])
			} else {
				// If no hostname was specified, the hostname should equal the ID
				require.Equal(t, string(*id2), props["hostname"])
			}
			// Verify the hostname prefix key/value is no longer in the props
			require.Nil(t, props["@hostname_prefix"])

		case VMAmazon:
			require.Equal(t, map[string]interface{}{
				"InstancePlugin": "terraform",
				"label1":         "value1",
				"label2":         "value2",
				"Name":           string(*id2),
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
				"name":                        string(*id1),
			},
		}
		inst2 = instance.Description{
			ID: *id2,
			Tags: map[string]string{
				"terraform_demo_swarm_mgr_sl": "",
				"label1":                      "value1",
				"label2":                      "value2",
				"name":                        string(*id2),
			},
		}
	case VMAmazon:
		inst1 = instance.Description{
			ID: *id1,
			Tags: map[string]string{
				"InstancePlugin": "terraform",
				"label1":         "value1",
				"label2":         "value2",
				"Name":           string(*id1),
			},
		}
		inst2 = instance.Description{
			ID: *id2,
			Tags: map[string]string{
				"InstancePlugin": "terraform",
				"label1":         "value1",
				"label2":         "value2",
				"Name":           string(*id2),
			},
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

	buff, err = ioutil.ReadFile(tfPath)
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
		}), conv(props["tags"].([]interface{})))
	case VMAmazon:
		require.Equal(t, map[string]interface{}{
			"InstancePlugin": "terraform",
			"label1":         "changed1",
			"label2":         "value2",
			"label3":         "value3",
			"Name":           string(*id2),
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

	// Destroy, then none should match
	err = terraform.Destroy(*id2)
	require.NoError(t, err)

	list, err = terraform.DescribeInstances(map[string]string{"label1": "changed1"}, false)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{}, list)
}

func conv(a []interface{}) []string {
	sa := make([]string, len(a))
	for i, x := range a {
		sa[i] = x.(string)
	}
	sort.Strings(sa)
	return sa
}

package main

import (
	"encoding/base64"
	"encoding/json"
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
          ]
        }
    }
  }
}
`)

	// Test a softlayer_virtual_guest without an @hostname_prefix
	run(t, "softlayer_virtual_guest", `
{
  "resource" : {
    "ibmcloud_infra_file_storage": {
      "csy_test_file_storage1": {
        "iops" : 0.25,
        "type" : "Endurance",
        "datacenter" : "dal10",
        "capacity" : 20
      }
    },
    "softlayer_virtual_guest" : {
       "host" : {
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
	  "ssh_key_ids": [ "${data.softlayer_ssh_key.public_key.id}" ]
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
			]
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

	instanceSpec := instance.Spec{
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

	id, err := terraform.Provision(instanceSpec)
	require.NoError(t, err)

	tfPath := filepath.Join(dir, string(*id)+".tf.json")
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

	_, vms := firstInMap(m["resource"].(map[string]interface{}))
	_, v := firstInMap(vms.(map[string]interface{}))
	value, _ := v.(map[string]interface{})

	switch vmType {
	case VM_SL:
		require.Equal(t, conv([]interface{}{
			"terraform_demo_swarm_mgr_sl",
			"label1:value1",
			"label2:value2",
			"name:" + string(*id),
		}), conv(props["tags"].([]interface{})))
		require.Equal(t, instanceSpec.Init, props["user_metadata"])

		// If a hostname was specified, the expectation is that the hostname is appended with the timestamp from the ID
		if value["@hostname_prefix"] != nil && strings.Trim(value["@hostname_prefix"].(string), " ") != "" {
			newID := strings.Replace(string(*id), "instance-", "", -1)
			expectedHostname := "softlayer-hostname-" + newID
			require.Equal(t, expectedHostname, props["hostname"])
		} else {
			// If no hostname was specified, the hostname should equal the ID
			require.Equal(t, string(*id), props["hostname"])
		}
		// Verify the hostname prefix key/value is no longer in the props
		require.Nil(t, props["@hostname_prefix"])

	case VM_AWS:
		require.Equal(t, map[string]interface{}{
			"InstancePlugin": "terraform",
			"label1":         "value1",
			"label2":         "value2",
			"Name":           string(*id),
		}, props["tags"])
		require.Equal(t, base64.StdEncoding.EncodeToString([]byte(instanceSpec.Init)), props["user_data"])
	}

	// label resources
	err = terraform.Label(*id, map[string]string{
		"label1": "changed1",
		"label3": "value3",
	})

	buff, err = ioutil.ReadFile(tfPath)
	require.NoError(t, err)

	any = types.AnyBytes(buff)

	parsed := TFormat{}
	err = any.Decode(&parsed)
	require.NoError(t, err)

	vmType, _, props, err = FindVM(&parsed)
	switch vmType {
	case VM_SL:
		require.Equal(t, conv([]interface{}{
			"terraform_demo_swarm_mgr_sl",
			"label1:changed1",
			"label2:value2",
			"label3:value3",
			"name:" + string(*id),
		}), conv(props["tags"].([]interface{})))
	case VM_AWS:
		require.Equal(t, map[string]interface{}{
			"InstancePlugin": "terraform",
			"label1":         "changed1",
			"label2":         "value2",
			"label3":         "value3",
			"Name":           string(*id),
		}, props["tags"])
	}

	list, err := terraform.DescribeInstances(map[string]string{"label1": "changed1"}, false)
	require.NoError(t, err)

	switch vmType {
	case VM_SL:
		require.Equal(t, []instance.Description{
			{
				ID: *id,
				Tags: map[string]string{
					"terraform_demo_swarm_mgr_sl": "",
					"label1":                      "changed1",
					"label2":                      "value2",
					"label3":                      "value3",
					"name":                        string(*id),
				},
			},
		}, list)
	case VM_AWS:
		require.Equal(t, []instance.Description{
			{
				ID: *id,
				Tags: map[string]string{
					"InstancePlugin": "terraform",
					"label1":         "changed1",
					"label2":         "value2",
					"label3":         "value3",
					"Name":           string(*id),
				},
			},
		}, list)
	}

	err = terraform.Destroy(*id)
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

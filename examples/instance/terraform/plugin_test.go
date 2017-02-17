package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestUsage(t *testing.T) {
	run(t, "softlayer_virtual_guest", `
{
        "type": "softlayer_virtual_guest",
        "value": {
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
`)

	run(t, "aws_instance", `
{
        "type" : "aws_instance",
        "value" : {
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
}`)
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
	parsed := TFormat{}
	err = any.Decode(&parsed)
	require.NoError(t, err)
	require.NotNil(t, parsed.Resource)

	props := parsed.Resource[resourceType][string(*id)]

	switch resourceType {
	case "softlayer_virtual_guest":
		require.Equal(t, conv([]interface{}{
			"terraform_demo_swarm_mgr_sl",
			"label1:value1",
			"label2:value2",
			"Name:" + string(*id),
		}), conv(props["tags"].([]interface{})))
		require.Equal(t, instanceSpec.Init, props["user_metadata"])
	case "aws_instance":
		require.Equal(t, map[string]interface{}{
			"InstancePlugin": "terraform",
			"label1":         "value1",
			"label2":         "value2",
			"Name":           string(*id),
		}, props["tags"])
		require.Equal(t, instanceSpec.Init, props["user_data"])
	}

	// label resources
	err = terraform.Label(*id, map[string]string{
		"label1": "changed1",
		"label3": "value3",
	})

	buff, err = ioutil.ReadFile(tfPath)
	require.NoError(t, err)

	any = types.AnyBytes(buff)
	parsed = TFormat{}
	err = any.Decode(&parsed)
	require.NoError(t, err)

	props = parsed.Resource[resourceType][string(*id)]
	switch resourceType {
	case "softlayer_virtual_guest":
		require.Equal(t, conv([]interface{}{
			"terraform_demo_swarm_mgr_sl",
			"label1:changed1",
			"label2:value2",
			"label3:value3",
			"Name:" + string(*id),
		}), conv(props["tags"].([]interface{})))
	case "aws_instance":
		require.Equal(t, map[string]interface{}{
			"InstancePlugin": "terraform",
			"label1":         "changed1",
			"label2":         "value2",
			"label3":         "value3",
			"Name":           string(*id),
		}, props["tags"])
	}

	list, err := terraform.DescribeInstances(map[string]string{"label1": "changed1"})
	require.NoError(t, err)

	switch resourceType {
	case "softlayer_virtual_guest":
		require.Equal(t, []instance.Description{
			{
				ID: *id,
				Tags: map[string]string{
					"terraform_demo_swarm_mgr_sl": "",
					"label1":                      "changed1",
					"label2":                      "value2",
					"label3":                      "value3",
					"Name":                        string(*id),
				},
			},
		}, list)
	case "aws_instance":
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

	list, err = terraform.DescribeInstances(map[string]string{"label1": "changed1"})
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

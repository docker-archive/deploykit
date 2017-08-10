InfraKit Instance Plugin - Terraform
=====================================

A [reference](/README.md#reference-implementations) implementation of an Instance Plugin that creates machines
using [Terraform](https://www.terraform.io).  In this concept, InfraKit provides the active group management while
Terraform performs the functions of resource provisioning.

This poc is adapted from the [`aws-two-tier`](https://github.com/hashicorp/terraform/tree/master/examples/aws-two-tier)
example from the Terraform project. There are some minor changes:

  + Variables that required on-screen user interaction (for setting public keys) have been removed
  and replaced with the `key_name` parameter in the provisioning config.
  + The ELB has been removed to keep the example simple -- registration of backends for ELB is
  coming in the future.
  + Removed the output variables which depended on the ELB.

The entire integration can be seen in the file [`plugin.go`](plugin.go).  Some interesting points:

  + Provisioning via Terraform is simply creating a `.tf.json` file on disk and calling `terraform apply`.
  + `terraform apply` uses a local lock file to make sure only one process is access and updating the
  `.tfstate` file.  This is to avoid concurrent access which can sometimes corrupt the state file.
  In the end, the design is simplified:  we only need to write the files to disk and a dedicated
  goroutine continuously calls `terraform apply`.  
  + The plugin auto-generates names for resources so user doesn't have to.
  + Destroying an instance is simply removing the `.tf.json` file and calling `terraform apply`.
  + Describing instances simply go through all the `instance-X.tf.json` files on disk and returns them
  as the result. This is a pretty simplistic integration but it is more correct than writing files
  and then reading the `tfstate` file.  The generated tf json file instances are essentially
  "promises" that terraform will eventually provision. A tighter integration is worth investigation.

[Here](cattle_demo.md) is a simple demo of using Terraform with group and Vanilla plugins.

## Configuration

This directory contains a `main.tf` that builds a VPC with subnet on AWS.  EC2 instances
(or `aws_instance` resource) are then added to the config as separate files when the plugin provisions
the resource.  For an EC2 instance this is a valid `.tf.json`:

```json
{
    "resource": {
      "aws_instance": {
        "instance-1474957906": {
          "ami": "${lookup(var.aws_amis, var.aws_region)}",
          "instance_type": "m1.small",
          "key_name": "PUBKEY",
          "subnet_id": "${aws_subnet.default.id}",
          "tags": {
            "InstancePlugin": "terraform",
            "Tier": "web",
            "other": "values"
          },
          "user_data": "#!/bin/sh; echo 'hello world'",
          "vpc_security_group_ids": [
            "${aws_security_group.default.id}"
          ],
          "connection": {
            "user": "ubuntu"
          }
        }
      }
    }
  }
```

Terraform's configuration schema requires user assignment of names for individual resources, while
InfraKit operates on groups of them.  So we changed the JSON format slightly to require only the
resource type name (e.g. `aws_instance`).  This is the spec for the instance plugin:

```json
{
    "Properties" : {
      "resource" : {
        "aws_instance": {
          "ami" : "${lookup(var.aws_amis, var.aws_region)}",
          "instance_type" : "m1.small",
          "key_name": "PUBKEY",
          "vpc_security_group_ids" : ["${aws_security_group.default.id}"],
          "subnet_id": "${aws_subnet.default.id}",
          "tags" :  {
              "Tier" : "web",
              "InstancePlugin" : "terraform"
          },
          "connection" : {
              "user" : "ubuntu"
          }
        }
      }
    },
    "Tags" : {
        "other" : "values"
    },
    "Init" : "#!/bin/sh; sudo apt-get -y update; sudo apt-get -y install nginx; sudo service nginx start"
}
```

For validation of input, the plugin only checks for validity of the JSON blob in the `Properties` field
above.  Note that there are two properties of importance: `type` and `value`.  `type` is the resource
type in terraform (in this case `aws_instance`, but can also be other resource types, as long as it's
something that makes sense with the environment provisioned in `main.tf`.

When provisioning, the plugin assigns a name first and then generates a valid `tf.json`.  `terraform apply`
is run continuously in the background so as soon as new files are deposited, Terraform will provision
and update its state.  When an instance is removed, Terraform will do the same by destroying the instance
and update its state.


## Running

Begin by building plugin [binaries](/README.md#binaries).

The supported fields are:
* `dir`: Directory that will be used to contain the `tfstate` and `tf.json` files
* `poll-interval`: Frequency that `terraform apply` is invoked; note that it is only invoked on the leader manager (unless `standalone=true`, see below)
* `standalone`: If `true` then manager leadership is not verified prior to invoking `terraform apply` (default is `false`)

The plugin also supports importing an existing resource into terraform; this can be used to import the initial manager into terraform. Once the resource is imported into terraform, a corresponding `.tf.json` file is also created. The following optional fields are used for this purpose:
* `import-group-spec-url`: The group specification URL that contains a nested instance specification; the `.tf.json` file for the imported resource contains the properties in the instance specification
* `import-instance-id`: The ID of the instance to import
* `import-group-id`: Optional group ID that the imported resource should be tagged with

The plugin also checks to make sure it can call `terraform`. Install Terraform [here](https://www.terraform.io/downloads.html) if you haven't done so.

See the [CLI Doc](/cmd/infrakit/README.md) for details on accessing the instance plugin via CLI.

Start the plugin:

```shell
$ build/infrakit-instance-terraform --dir=./pkg/provider/terraform/instance/aws-two-tier/
INFO[0000] Listening at: ~/.infrakit/plugins/instance-terraform
```

Be sure to verify that the plugin is [discoverable](/cmd/infrakit/README.md#list-plugins).

Now lets try to validate something.  Instead of reading from stdin we are loading from a file
to avoid problems with bad bash substitution beacuse Terrafrom configs use `$` to indicate variables.

```shell
$ cat pkg/provider/terraform/instance/aws-two-tier/instance-plugin-properties.json
{
    "type" : "aws_instance",
    "value" : {
        "ami" : "${lookup(var.aws_amis, var.aws_region)}",
        "instance_type" : "m1.small",
        "key_name": "PUBKEY",
        "vpc_security_group_ids" : ["${aws_security_group.default.id}"],
        "subnet_id": "${aws_subnet.default.id}",
        "tags" :  {
            "Tier" : "web",
            "InstancePlugin" : "terraform"
        },
        "connection" : {
            "user" : "ubuntu"
        }
    }
}
$ build/infrakit instance --name instance-terraform validate pkg/provider/terraform/instance/aws-two-tier/instance-plugin-properties.json
validate:ok
```

Now we can provision:

```shell
$ cat pkg/provider/terraform/instance/aws-two-tier/instance-plugin-spec.json
{
    "Properties" : {
        "type" : "aws_instance",
        "value" : {
            "ami" : "${lookup(var.aws_amis, var.aws_region)}",
            "instance_type" : "m1.small",
            "key_name": "PUBKEY",
            "vpc_security_group_ids" : ["${aws_security_group.default.id}"],
            "subnet_id": "${aws_subnet.default.id}",
            "tags" :  {
                "Tier" : "web",
                "InstancePlugin" : "terraform"
            },
            "connection" : {
                "user" : "ubuntu"
            }
        }
    },
    "Tags" : {
        "other" : "values"
    },
    "Init" : "#!/bin/sh; sudo apt-get -y update; sudo apt-get -y install nginx; sudo service nginx start"
}
$ build/infrakit instance --name instance-terraform provision pkg/provider/terraform/instance/aws-two-tier/instance-plugin-spec.json
instance-1475004829
```

Now list them.

```shell
$ build/infrakit instance --name instance-terraform describe
ID                            	LOGICAL                       	TAGS
instance-1475004829           	  -                           	other=values,provisioner=infrakit-terraform-example,InstancePlugin=terraform,Name=instance-1475004829,Tier=web
```

You should verify that the instances are indeed created in the AWS Console.
In AWS Console you can filter by tag `provisioner` with value `infrakit-terraform-example`:

![RSG Screenshot](example.png)

Now destroy the instance:

```shell
$ build/infrakit instance --name instance-terraform destroy instance-1475004829
destroyed instance-1475004829
$ build/infrakit instance --name instance-terraform describe
ID                            	LOGICAL                       	TAGS
```

![RSG Screenshot](terminated.png)

## Technical Details

Terraform tracks the state of resources (corresponding to `.tf.json` files) in the `.tfstate` file.

In order to detect resources that have been removed outside of Infrakit (for example, if an instance is
removed from the Cloud provider natively), the terraform instance plugin creates an intermediate
`tf.json.new` file on `Provision`; these files are **not processed** during a `terraform apply`. However,
the terraform instance plugin **does process**  both `tf.json` and `tf.json.new` files in the
`DescribeInstances`, `Destroy`, and `Label` functions.

The terraform instance plugin has a dedicated goroutine (running at a configurable frequency) that
executes the following:

1. Acquires a file lock (protects access to all `tf.json` and `tf.json.new` files)
2. Executes `terraform refresh` to update the `.tfstate` file
3. Executes `terraform state list` to list all resources in the state file
4. Removes any `tf.json` file that corresponds to an instance that is **not** in the state file
5. Renames all `tf.json.new` files to `tf.json` files so that a `terraform apply` will process them
6. Releases file lock (from step 1)
7. Executes `terraform apply -no-refresh`

But naming new files with the `.tf.json.new` suffix in the `Provision` flow, the plugin can differentiate
between orphaned resources and those queued up for creation.

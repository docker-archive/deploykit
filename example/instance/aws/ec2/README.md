InfraKit Instance Plugin - AWS EC2 Instance
===========================================

This is a simple instance plugin that will provision an EC2 instance on AWS.  A few interesting
things:

  + It is able to "discover" the region, subnet, security groups, instance type, image of the current
  instance where the plugin is running and use those as values if they are not specified in the
  input.  This makes it easy to specify minimal input and effectively 'clone' the instance many
  times if used with the group plugin.
  + Of course, you can override it.  See the example [`instance-properties.json`](instance-properties.json)
  as an example -- it just overrides the defaults with the instance type `t2.nano`.
  + While you can specify the AWS credentials on the command line when starting the plugin,
  it's best that you run the plugin on an AWS instance with a given IAM instance role.  This way no credentials
  are ever exposed anywhere in the command lines, etc.

## Name of the plugin

The name can be set based on the unix socket in the `listen` flag when starting up the plugin.  Currently,
the name defaults to `aws-ec2` (meaning you'd see a file named `aws-ec2.sock` in `/run/infrakit/plugins` or
whatever the plugin discovery directory you set (as part of the path of the `unix://` url in `listen` flag).


## Input to the Plugin -- the `Properties` block

The struct [`CreateInstanceRequest`](/plugin/instance/aws/ec2/plugin.go) is the Golang struct that is unmarshaled
from the opaque blob value of the field `Properties` in other JSON structure that uses this plugin.  It looks like
```go
type CreateInstanceRequest struct {
	DiscoverDefaults  bool
	Tags              map[string]string
	RunInstancesInput ec2.RunInstancesInput
}
```

Note that it's just made up of 
  
  + a flag `DiscoverDefaults` which will turn on discovery and set some default values
so you don't have to.
  + An associative array (dictionary or map) of the tags you want to use
  + The [`RunInstancesInput`](http://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#RunInstancesInput)
   of the AWS EC2 API, in JSON form.

Here is an example, 

```json
{
    "DiscoverDefaults" : true,
    "Tags" : {
	"env" : "dev",
	"instance-plugin" : "aws-ec2"
    },
    "RunInstancesInput" : {
	"InstanceType" : "t2.nano",
        "KeyName" : "some-key-name"
    }
}
```

So when you put this in a larger JSON, say using this with a Group plugin, the config JSON would look like:

```json
{
    "ID": "aws_ec2_demo",
    "Properties": {
        "Instance" : {
            "Plugin": "aws-ec2",
            "Properties": {
                "DiscoverDefaults" : true,
                "Tags" : {
                    "env" : "dev",
                    "instance-plugin" : "aws-ec2"
                },
                "RunInstancesInput" : {
                    "InstanceType" : "t2.micro"
                }
            }
        },
	"Flavor" : {
            "Plugin": "flavor-vanilla",
            "Properties": {
		"Size" : 5,
		"UserData" : [
                    "sudo apt-get update -y",
                    "sudo apt-get install -y nginx",
                    "sudo service nginx start"
		]
            }
	}
    }
}
```
In the example above, the vanila flavor plugin is used.  So we are just managing cattle where all instances
look alike. 

To have the Group plugin watch this group, first make sure the plugins are all running (see tutorial and examples elsewhere).
Then, do this:

```
$ infrakit/cli group watch group.json
```

With this minimal setup, you can set up a group in the same subnet, using the same AMI, with the same security groups.

## Environment Discovery

In contrast to the Terrform plugin which has a Terraform script (`main.tf`) that can provision a whole new VPC and
subnets (which you can remove and use your own), this plugin can provision instances right into your existing subnet
if you set the `DiscoverDefault` flag.  This is handy and requires minimal setup other than running the plugin on
an instance with the correct IAM instance role.  You can always provide your own values by setting the fields in the
`RunInstancesInput` struct.

This plugin uses the [AWS Instance Metadata Service](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.htm)
to discover the local environment so you don't have to tell it.  
It sets the following to discovered values if the values are missing in the `Properties` block:

  + The region (discovered unless set at start up of plugin)
  + The Instance Type  (`InstanceType *string`)
  + The AMI id (`ImageId *string`)
  + The Subnet (`SubnetId *string`)
  + The Security groups (`SecurityGroupIds []*string`)
  
Note that the IAM instance role is not discovered and set (the `IamInstanceProfile` field in the `RunInstancesInput`
struct), because provisioned instances shouldn't have the same level of access to your infrastructure as the controller
node where the plugin runs.  If you feel differently, you can always set the structure in the JSON.  

It also doesn't discover the SSH key name.  Set the `KeyName` field manually or skip it if you don't need SSH access.
SSH access is rarely required if user data (the Init) can do all the configuration (install software, start up daemons -- the
things that a Flavor plugin can automate -- like for Docker Swarm worker nodes).  
It's also a good practice, especially for the immutable pattern we would like to encourage.  Things get harder to manage
at scale when your remote SSH calls to configure nodes work half-way through... due to errors in your scripts or network, etc.




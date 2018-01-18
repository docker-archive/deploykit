# InfraKit AWS Provider

[InfraKit](https://github.com/docker/infrakit) plugins for creating and managing resources in Amazon AWS.

## Instance plugin

An InfraKit instance plugin is provided, which creates Amazon EC2 instances.

### Building and running

To build the AWS Instance plugin, run `make binaries`.  The plugin binary will be located at
`./build/infrakit plugin start aws --log 5`.

At a minimum, the plugin requires the AWS region to use.  However, this can be inferred from instance metadata when the
plugin is running within EC2.  In other cases, specify the `--region` argument:
```console
$ INFRAKIT_AWS_REGION=us-west-2 build/infrakit plugin start aws --log 5
INFO[11-06|16:56:32] config                                   module=cli/plugin url= fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:56:32] Launching                                module=cli/plugin kind=aws name=aws fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:56:32] Starting plugin                          module=core/launch executor=inproc key=aws name=aws exec=inproc fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-06|16:56:32] Starting                                 module=aws/metadata context="&{update:<nil> poll:60000000000 templateURL: templateOptions:{DelimLeft: DelimRight: CustomizeFetch:<nil> Stderr:<nil> MultiPass:false CacheDir:} stop:0xc42021c180 stackName: clients:{Cfn:0xc42000e8f0 Ec2:0xc42000e900 Asg:0xc42000e910} impl:<nil>}" poll=1m0s fn=github.com/docker/infrakit/pkg/provider/aws/plugin/metadata.(*Context).start
INFO[11-06|16:56:32] Object is an event producer              module=rpc/server object="&{plugin:<nil> typedPlugins:map[ec2-instance:0xc42053a4c0]}" discover=/Users/infrakit/.infrakit/plugins/aws fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[0000] Start monitoring instances 0xc42021cb40
INFO[11-06|16:56:32] Listening                                module=rpc/server discover=/Users/infrakit/.infrakit/plugins/aws fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[11-06|16:56:32] Waiting for startup                      module=core/launch key=aws name=aws config="{\n\"Kind\": \"aws\",\n\"Options\": {\n\"Namespace\": {},\n\"ELBNames\": [\n\"\"\n],\n\"Region\": \"ca-central-1\",\n\"AccessKeyID\": \"\",\n\"SecretAccessKey\": \"\",\n\"SessionToken\": \"\",\n\"Retries\": 0,\n\"Debug\": false,\n\"Template\": \"\",\n\"TemplateOptions\": {\n\"DelimLeft\": \"\",\n\"DelimRight\": \"\",\n\"MultiPass\": false,\n\"CacheDir\": \"\"\n},\n\"StackName\": \"\",\n\"PollInterval\": \"1m0s\"\n}\n}" as=aws fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-06|16:56:32] Done waiting on plugin starts            module=cli/plugin fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:56:32] PID file created                         module=run path=/Users/infrakit/.infrakit/plugins/aws.pid fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-06|16:56:32] Server started                           module=run discovery=/Users/infrakit/.infrakit/plugins/aws fn=github.com/docker/infrakit/pkg/run.run.func1
```

### Example

To continue with an example, we will use the [default](https://github.com/docker/infrakit/tree/master/cmd/group) Group
plugin:
```console
$ build/infrakit plugin start group
INFO[11-06|16:58:10] config                                   module=cli/plugin url= fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:58:10] Launching                                module=cli/plugin kind=group name=group-stateless fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:58:10] Starting plugin                          module=core/launch executor=inproc key=group name=group-stateless exec=inproc fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-06|16:58:10] Object is an event producer              module=rpc/server object=&{keyed:0xc4201862f8} discover=/Users/infrakit/.infrakit/plugins/group-stateless fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[11-06|16:58:10] Listening                                module=rpc/server discover=/Users/infrakit/.infrakit/plugins/group-stateless fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[11-06|16:58:10] Waiting for startup                      module=core/launch key=group name=group-stateless config="{\n\"Kind\": \"group\",\n\"Options\": {\n\"PollInterval\": \"10s\",\n\"MaxParallelNum\": 0,\n\"PollIntervalGroupSpec\": \"10s\",\n\"PollIntervalGroupDetail\": \"10s\"\n}\n}" as=group-stateless fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-06|16:58:10] Done waiting on plugin starts            module=cli/plugin fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:58:10] PID file created                         module=run path=/Users/infrakit/.infrakit/plugins/group-stateless.pid fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-06|16:58:10] Server started                           module=run discovery=/Users/infrakit/.infrakit/plugins/group-stateless fn=github.com/docker/infrakit/pkg/run.run.func1
```

and the [Vanilla](https://github.com/docker/infrakit/tree/master/pkg/example/flavor/vanilla) Flavor plugin:.
```console
$ build/infrakit plugin start vanilla
INFO[11-06|16:59:01] config                                   module=cli/plugin url= fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:59:01] Launching                                module=cli/plugin kind=vanilla name=vanilla fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:59:01] Starting plugin                          module=core/launch executor=inproc key=vanilla name=vanilla exec=inproc fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-06|16:59:01] Listening                                module=rpc/server discover=/Users/infrakit/.infrakit/plugins/vanilla fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[11-06|16:59:01] Waiting for startup                      module=core/launch key=vanilla name=vanilla config="{\n\"Kind\": \"vanilla\",\n\"Options\": {\n\"DelimLeft\": \"\",\n\"DelimRight\": \"\",\n\"MultiPass\": true,\n\"CacheDir\": \"\"\n}\n}" as=vanilla fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-06|16:59:01] Done waiting on plugin starts            module=cli/plugin fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:59:01] PID file created                         module=run path=/Users/infrakit/.infrakit/plugins/vanilla.pid fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-06|16:59:01] Server started                           module=run discovery=/Users/infrakit/.infrakit/plugins/vanilla fn=github.com/docker/infrakit/pkg/run.run.func1
```

We will use a basic configuration that creates a single instance:
```console
$ cat << EOF > aws-vanilla.json
{
  "ID": "aws-example",
  "Properties": {
    "Allocation": {
      "Size": 1
    },
    "Instance": {
      "Plugin": "aws/ec2-instance",
      "Properties": {
        "RunInstancesInput": {
          "ImageId": "ami-4926fd29",
          "KeyName": "my-laptop",
          "Placement": {
            "AvailabilityZone": "us-west-2a"
          },
          "SecurityGroupIds": ["sg-57411931"]
        },
        "Tags": {
          "Name": "infrakit-example"
        }
      }
    },
    "Flavor": {
      "Plugin": "vanilla",
      "Properties": {
        "Init": [
          "sh -c \"echo 'Hello, World!' > /hello\""
        ]
      }
    }
  }
}
EOF
```

For the structure of `RunInstancesInput`, please refer to [the document of AWS SDK for Go](https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#RunInstancesInput).

Note that you will need to replace the `KeyName` with an
[SSH key pair](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html) you have access to, and the
`SecurityGroups` with a group available in your VPC.  For the purposes of this example, it will be helpful to select
a [Security Group](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-network-security.html) that you can access
via [SSH](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AccessingInstancesLinux.html).

The instance type is set to `m1.small` by default. Note that you cannot use HVM images for `m1.small`.

Finally, instruct the Group plugin to start watching the group:
```console
$ build/infrakit group-stateless commit aws-vanilla.json
Committed aws-example: Managing 1 instances
```

In the console running the Group plugin, we will see input like the following:
```
INFO[1219] Committing group aws-example (pretend=false)
INFO[1219] Adding 1 instances to group to reach desired 1
INFO[1219] Created instance i-ba0412a2 with tags map[infrakit.config.hash:dUBtWGmkptbGg29ecBgv1VJYzys= infrakit.group:aws-example]
```

Additionally, the CLI will report the newly-created instance:
```console
$ build/infrakit group-stateless inspect aws-example
ID                             	LOGICAL                        	TAGS
i-ba0412a2                     	172.31.41.13                   	Name=infrakit-example,infrakit.config.hash=dUBtWGmkptbGg29ecBgv1VJYzys=,infrakit.group=aws-example
```

Retrieve the IP address of the host from the AWS console, and use SSH to verify that our shell code ran:

```console
$ ssh ubuntu@55.55.55.55 cat /hello
Hello, World!
```

### Plugin properties

The plugin expects properties in the following format:
```json
{
  "Tags": {
  },
  "RunInstancesInput": {
  }
}
```

The `Tags` property is a string-string mapping of EC2 instance tags to include on all instances that are created.
`RunInstancesInput` follows the structure of the type by the same name in the
[AWS go SDK](http://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#RunInstancesInput).


#### AWS API Credentials

The plugin can use API credentials from several sources.
- config file:
  see [AWS docs](http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html#cli-config-files)
- EC2 instance metadata:
  see [AWS docs](http://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2.html)

Additional credentials sources are supported, but are not generally recommended as they are less secure:
- command line arguments: `--session-token`, or  `--access-key-id` and `--secret-access-key`
- environment variables:
  see [AWS docs](http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html#cli-environment)


## Reporting security issues

The maintainers take security seriously. If you discover a security issue,
please bring it to their attention right away!

Please **DO NOT** file a public issue, instead send your report privately to
[security@docker.com](mailto:security@docker.com).

Security reports are greatly appreciated and we will publicly thank you for it.
We also like to send gifts—if you're into Docker schwag, make sure to let
us know. We currently do not offer a paid security bounty program, but are not
ruling it out in the future.


## Copyright and license

Copyright © 2016 Docker, Inc. All rights reserved, except as follows. Code
is released under the Apache 2.0 license. The README.md file, and files in the
"docs" folder are licensed under the Creative Commons Attribution 4.0
International License under the terms and conditions set forth in the file
"LICENSE.docs". You may obtain a duplicate copy of the same license, titled
CC-BY-SA-4.0, at http://creativecommons.org/licenses/by/4.0/.

Infrakit Extensible CLI - Examples
==================================

Any subfolders here will be added as a command, as in any file with the `.ikc` extension.
The directory tree will be followed recursively and commands will follow the hierarchy of
the file system from the point set by the `INFRAKIT_CLI_DIR` environment variable.

For example, if we point the enviroment variable to this directory, the folder `build` will show
up as a command with child commands, while the file `CreateCluster.ikc` will show up as a command.


```
~/infrakit$ export INFRAKIT_CLI_DIR=$PWD/pkg/cli/examples
~/infrakit$ infrakit -h
infrakit cli


 ___  ________   ________ ________  ________  ___  __    ___  _________
|\  \|\   ___  \|\  _____\\   __  \|\   __  \|\  \|\  \ |\  \|\___   ___\
\ \  \ \  \\ \  \ \  \__/\ \  \|\  \ \  \|\  \ \  \/  /|\ \  \|___ \  \_|
 \ \  \ \  \\ \  \ \   __\\ \   _  _\ \   __  \ \   ___  \ \  \   \ \  \
  \ \  \ \  \\ \  \ \  \_| \ \  \\  \\ \  \ \  \ \  \\ \  \ \  \   \ \  \
   \ \__\ \__\\ \__\ \__\   \ \__\\ _\\ \__\ \__\ \__\\ \__\ \__\   \ \__\
    \|__|\|__| \|__|\|__|    \|__|\|__|\|__|\|__|\|__| \|__|\|__|    \|__|


Usage:
  infrakit [command]

Available Commands:
  CreateCluster CreateCluster
  aws           Manage AWS resources
  event         Access event exposed by infrakit plugins
  flavor        Access flavor plugin
  group         Access group plugin
  info          print plugin info
  instance      Access instance plugin
  manager       Access the manager
  metadata      Access metadata exposed by infrakit plugins
  plugin        Manage plugins
  resource      Access resource plugin
  template      Render an infrakit template
  util          Utilties
  version       print build version information

Flags:
      --alsologtostderr                  log to standard error as well as files
  -H, --host stringSlice                 host list. Default is local sockets
      --httptest.serve string            if non-empty, httptest.NewServer serves on this address and blocks
      --log int                          log level (default 4)
      --log-caller                       include caller function (default true)
      --log-format string                log format: logfmt|term|json (default "term")
      --log-stack                        include caller stack
      --log-stdout                       log to stdout
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --logtostderr                      log to standard error instead of files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          log level for V logs
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging

Use "infrakit [command] --help" for more information about a command.
```

## Command Hierarchy follows Files Hierarchy

In this example, the folder `aws` has two `.ikc` files:

```shell
~/infrakit$ tree examples/cli/aws
examples/cli/aws
├── provision-instance.ikc
└── start-plugin.ikc
```
This will show up as

```
~/infrakit$ infrakit aws -h


___  ________   ________ ________  ________  ___  __    ___  _________
|\  \|\   ___  \|\  _____\\   __  \|\   __  \|\  \|\  \ |\  \|\___   ___\
\ \  \ \  \\ \  \ \  \__/\ \  \|\  \ \  \|\  \ \  \/  /|\ \  \|___ \  \_|
 \ \  \ \  \\ \  \ \   __\\ \   _  _\ \   __  \ \   ___  \ \  \   \ \  \
  \ \  \ \  \\ \  \ \  \_| \ \  \\  \\ \  \ \  \ \  \\ \  \ \  \   \ \  \
   \ \__\ \__\\ \__\ \__\   \ \__\\ _\\ \__\ \__\ \__\\ \__\ \__\   \ \__\
    \|__|\|__| \|__|\|__|    \|__|\|__|\|__|\|__|\|__| \|__|\|__|    \|__|


Manage AWS resources

Usage:
  infrakit aws [command]

Available Commands:
  provision-instance provision-instance
  start-plugin       start-plugin

Global Flags:
  -H, --host stringSlice        host list. Default is local sockets
      --httptest.serve string   if non-empty, httptest.NewServer serves on this address and blocks
      --log int                 log level (default 4)
      --log-caller              include caller function (default true)
      --log-format string       log format: logfmt|term|json (default "term")
      --log-stack               include caller stack
      --log-stdout              log to stdout

Use "infrakit aws [command] --help" for more information about a command.

```

## CLI Flags

Each file that corresponds to a command is a Golang template.  In the template, you can call functions to
bind to command line flags or to prompt the user.  For example, the file `CreateCluster.ikc` looks like:

```
#!/bin/bash

{{/* The directive here tells infrakit to run this script with sh:  =% sh %=  */}}

{{/* The function 'flag' will create a flag in the CLI; the function 'prompt' will ask user for input */}}

{{ $doCommit := flag "commit" "bool" "true to commit" false }}
{{ $clusterName := flag "cluster-name" "string" "the name of the cluster" "swarm" }}
{{ $clusterSize := flag "size" "int" "the size of the cluster" 20 }}

{{ $user := prompt "Please enter your user name" "string" }}

{{/* An example here where we expose a flag and if not set, ask the user */}}
{{ $instanceType := flag "instance-type" "string" "VM instance type" | prompt "Please specify vm instance type:" "string"}}

echo "Hello {{$user}}, the instance you selected is {{ $instanceType }}.  Creating {{$clusterSize}} instances."

# creating the instances

{{ range $i, $instance := until $clusterSize }}
create instance --cluster {{$clusterName}} --instance-type {{$instanceType}} id-{{$i}}
{{ end }}

{{ if $doCommit }}
commit
{{ end }}
```

Note that there are calls to the `flag` function:
```
{{ flag <flag_name> <type := bool|string|int|float> <description> [default] }}
```

This function will result in a command line flag bound to the command `CreateCluster`:

```
~/infrakit$ INFRAKIT_CLI_DIR=$PWD/pkg/cli/examples infrakit CreateCluster -h
CreateCluster


 ___  ________   ________ ________  ________  ___  __    ___  _________
|\  \|\   ___  \|\  _____\\   __  \|\   __  \|\  \|\  \ |\  \|\___   ___\
\ \  \ \  \\ \  \ \  \__/\ \  \|\  \ \  \|\  \ \  \/  /|\ \  \|___ \  \_|
 \ \  \ \  \\ \  \ \   __\\ \   _  _\ \   __  \ \   ___  \ \  \   \ \  \
  \ \  \ \  \\ \  \ \  \_| \ \  \\  \\ \  \ \  \ \  \\ \  \ \  \   \ \  \
   \ \__\ \__\\ \__\ \__\   \ \__\\ _\\ \__\ \__\ \__\\ \__\ \__\   \ \__\
    \|__|\|__| \|__|\|__|    \|__|\|__|\|__|\|__|\|__| \|__|\|__|    \|__|


Usage:
  infrakit CreateCluster [flags]

Flags:
      --cluster-name string    the name of the cluster (default "swarm")
      --commit                 true to commit
      --instance-type string   VM instance type
      --size int               the size of the cluster (default 20)

Global Flags:
      --alsologtostderr                  log to standard error as well as files
  -H, --host stringSlice                 host list. Default is local sockets
      --httptest.serve string            if non-empty, httptest.NewServer serves on this address and blocks
      --log int                          log level (default 4)
      --log-caller                       include caller function (default true)
      --log-format string                log format: logfmt|term|json (default "term")
      --log-stack                        include caller stack
      --log-stdout                       log to stdout
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --logtostderr                      log to standard error instead of files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          log level for V logs
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

## User Prompts

The `prompt` function in the template file will prompt the user to enter values at the command when it executes.
It has the form:
```
{{ prompt <message> <type:=string|bool|int|float64> }}
```

For example, running the `CreateCluster command will look like:

```
~/infrakit$ INFRAKIT_CLI_DIR=$PWD/pkg/cli/examples infrakit CreateCluster
Please enter your user name chungers
Please specify vm instance type: m2-small

```

It's possible to combine flag and prompt, so that if the value isn't passed in as a flag, the user is prompted:

```
{{ $instanceType := flag "instance-type" "string" "VM instance type" | prompt "Please specify vm instance type:" "string"}}
```
This line basically tries to get the value of `$instanceType` from the flag `instance-type` first and
if not present, prompts the user to enter with the specified message on screen.


## Backends

In each template file, in the comment line, you must declare which *backend* to use to actually run
this template:

```
{{/* Note the special delimiters around sh:  =% sh %=  */}}

```
This is parsed and used to determine what will actually interpret this rendered template.  We will be
adding different backends such as `sh`, `docker`, `runc`, `make`, etc.

For example, the file `aws/provision-instance.ikc` look like this:

```
# Input to create instance using the AWS instance plugin
{{/* =% sh %= */}}

{{ $user := flag "user" "string" "username" | prompt "Please enter your user name:" "string" }}
{{ $name := flag "name" "string" "name" | prompt "Name?" "string"}}
{{ $imageId := flag "ami" "string" "ami" | prompt "AMI?" "string"}}
{{ $instanceType := flag "instance-type" "string" "instance type" | prompt "Instance type?" "string"}}
{{ $keyName := flag "key" "string" "ssh key name" | prompt "SSH key?" "string"}}
{{ $az := flag "az" "string" "availability zone" | prompt "Availability zone?" "string"}}
{{ $subnetId := flag "subnet" "string" "subnet id" | prompt "Subnet ID?" "string"}}
{{ $securityGroupId := flag "security-group-id" "string" "security group id" | prompt "Security group ID?" "string" }}

infrakit --log 3 --log-stack --name instance-aws/ec2-instance instance provision -y - <<EOF

Tags:
  infrakit.name: {{ $name }}
  infrakit.created: {{ now | htmlDate }}
  infrakit.user: {{ $user }}

Init: |
  #!/bin/bash
  sudo apt-get update -y
  sudo apt-get install wget curl
  wget -q0- https://get.docker.com | sh

Properties:
  RunInstancesInput:
    BlockDeviceMappings: null
    DisableApiTermination: null
    EbsOptimized: null
    IamInstanceProfile: null
    ImageId: {{ $imageId }}
    InstanceInitiatedShutdownBehavior: null
    InstanceType: {{ $instanceType }}
    KeyName: {{ $keyName }}
    NetworkInterfaces:
    - AssociatePublicIpAddress: true
      DeleteOnTermination: true
      DeviceIndex: 0
      Groups:
      - {{ $securityGroupId }}
      NetworkInterfaceId: null
      SubnetId: {{ $subnetId }}
    Placement:
      Affinity: null
      AvailabilityZone: {{ $az }}
      Tenancy: null
    RamdiskId: null
    SubnetId: null
    UserData: null
  Tags:
    infrakit.name: {{ $name }}

EOF
```

Note the line `{{/* =% sh %= */}}` tells Infrakit to use `sh` as the backend.  Infrakit will render this template
interactively (since there are `prompts`).  When the template is successfully rendered, this shell script is
piped to `sh` for execution.  In this case, we are using heredocs `<<EOF` to pipe the YAML content to the
infrakit instance plugin, which will provision a new instance on AWS based on user's input.


## TO DO

- [ ] Implement additional backends like Docker / wire the hijacked connections
- [ ] SSH backend (can be implemented using `sh` today)

Currently `.ikc` are CLI command templates and thus support `flag` and `prompt` functions, while
the base `.ikt` templates rendered by servers don't have these functions.  However, we should unify these
via a pipeline mechanism similar to what was described here.  For example we could define something like

```
{{ $clusterName := ref "/cluster/name" | flag "cluster-name" "string" "Name of the cluster" | prompt "What's the name of cluster?" }}
```

For templates that are rendered by servers that do not have access to CLI or user tty, the values will obviously be
retrieved via the `ref` mechanism, which can be set via the `--global` flags or via sourcing of `.ikt` templates.
In environments where user interaction is possible, the user will be prompted if the value cannot be retrieved
via command line flags or via pre-sourced templates.

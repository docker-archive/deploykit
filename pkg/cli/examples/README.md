Infrakit Extensible CLI - Examples
==================================

Any subfolders here will be added as a command.  The directory tree will be followed recursively
and commands will follow the hierarchy of the file system from the point set by the
`INFRAKIT_CLI_DIR` environment variable.

For example, if we point the enviroment variable to this directory, the folder `build` will show
up as a command with child commands, while the file `CreateCluster` will show up as a command.


```
~/infrakit$ INFRAKIT_CLI_DIR=$PWD/pkg/cli/examples infrakit -h
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
  build         Infrakit build tools
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

In this example, the folder `build` has a nested folder `infrakit`, which has a file `make`.  This will
show up as

```
~/infrakit$ INFRAKIT_CLI_DIR=$PWD/pkg/cli/examples infrakit build infrakit -h
Self building infrakit


 ___  ________   ________ ________  ________  ___  __    ___  _________
|\  \|\   ___  \|\  _____\\   __  \|\   __  \|\  \|\  \ |\  \|\___   ___\
\ \  \ \  \\ \  \ \  \__/\ \  \|\  \ \  \|\  \ \  \/  /|\ \  \|___ \  \_|
 \ \  \ \  \\ \  \ \   __\\ \   _  _\ \   __  \ \   ___  \ \  \   \ \  \
  \ \  \ \  \\ \  \ \  \_| \ \  \\  \\ \  \ \  \ \  \\ \  \ \  \   \ \  \
   \ \__\ \__\\ \__\ \__\   \ \__\\ _\\ \__\ \__\ \__\\ \__\ \__\   \ \__\
    \|__|\|__| \|__|\|__|    \|__|\|__|\|__|\|__|\|__| \|__|\|__|    \|__|


Usage:
  infrakit build infrakit [command]

Available Commands:
  make        make

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

Use "infrakit build infrakit [command] --help" for more information about a command.
```

## CLI Flags

Each file that corresponds to a command is a Golang template.  In the template, you can call functions to
bind to command line flags or to prompt the user.  For example, the file `CreateCluster` looks like:

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

## TO DO

The current implementation doesn't actually do anything yet.  First we will implement the `sh` backend
where the content of the rendered template is piped to the `stdin` of `sh`.  We will also hook up the
output streams so that the user can interact with the actual process.  In the case of Docker as backend,
we plan to use the Docker API and use the hijacked connection returned from starting the container.

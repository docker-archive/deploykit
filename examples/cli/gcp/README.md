Creating Instance on GCP
========================

You can use flags on the command line or start the plugin interactively:

```shell
$ infrakit gcp start-plugin
Run as Docker container? n
What's the zone? us-central1-f
What's the name of the project? docker4x
Starting daemon
Tailing log
time="2017-04-05T00:34:40-07:00" level=debug msg="Using namespacemap[infrakit.scope:docker4x]"
time="2017-04-05T00:34:40-07:00" level=debug msg="Project: docker4x"
time="2017-04-05T00:34:40-07:00" level=debug msg="Zone: us-central1-f"
time="2017-04-05T00:34:40-07:00" level=info msg="Listening at: /Users/davidchung/.infrakit/plugins/instance-gcp"
time="2017-04-05T00:34:40-07:00" level=info msg="PID file at /Users/davidchung/.infrakit/plugins/instance-gcp.pid"
```

Now in another shell, create an instance, using the `provision-instance` command which is basically a 'script'
that is based on a YAML input and piped to `infrakit instance provision` itself:

```shell
$ infrakit gcp -h


___  ________   ________ ________  ________  ___  __    ___  _________
|\  \|\   ___  \|\  _____\\   __  \|\   __  \|\  \|\  \ |\  \|\___   ___\
\ \  \ \  \\ \  \ \  \__/\ \  \|\  \ \  \|\  \ \  \/  /|\ \  \|___ \  \_|
 \ \  \ \  \\ \  \ \   __\\ \   _  _\ \   __  \ \   ___  \ \  \   \ \  \
  \ \  \ \  \\ \  \ \  \_| \ \  \\  \\ \  \ \  \ \  \\ \  \ \  \   \ \  \
   \ \__\ \__\\ \__\ \__\   \ \__\\ _\\ \__\ \__\ \__\\ \__\ \__\   \ \__\
    \|__|\|__| \|__|\|__|    \|__|\|__|\|__|\|__|\|__| \|__|\|__|    \|__|


gcp

Usage:
  infrakit gcp [command]

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

Use "infrakit gcp [command] --help" for more information about a command.
```

```shell
$ infrakit gcp provision-instance -h


___  ________   ________ ________  ________  ___  __    ___  _________
|\  \|\   ___  \|\  _____\\   __  \|\   __  \|\  \|\  \ |\  \|\___   ___\
\ \  \ \  \\ \  \ \  \__/\ \  \|\  \ \  \|\  \ \  \/  /|\ \  \|___ \  \_|
 \ \  \ \  \\ \  \ \   __\\ \   _  _\ \   __  \ \   ___  \ \  \   \ \  \
  \ \  \ \  \\ \  \ \  \_| \ \  \\  \\ \  \ \  \ \  \\ \  \ \  \   \ \  \
   \ \__\ \__\\ \__\ \__\   \ \__\\ _\\ \__\ \__\ \__\\ \__\ \__\   \ \__\
    \|__|\|__| \|__|\|__|    \|__|\|__|\|__|\|__|\|__| \|__|\|__|    \|__|


provision-instance

Usage:
  infrakit gcp provision-instance [flags]

Flags:
      --disk-size int         Disk size in mb
      --machine-type string   Machine type
      --prefix string         Prefix to use
      --user string           owner

Global Flags:
  -H, --host stringSlice        host list. Default is local sockets
      --httptest.serve string   if non-empty, httptest.NewServer serves on this address and blocks
      --log int                 log level (default 4)
      --log-caller              include caller function (default true)
      --log-format string       log format: logfmt|term|json (default "term")
      --log-stack               include caller stack
      --log-stdout              log to stdout
```

Note that there are flags defined.  You can set the in the commandline, or set the interactively:

```shell
$ infrakit gcp provision-instance
Owner? dchung
Prefix for hostname: dchung
Disk size in MB [60]? 100
Machine type [n1-standard-1]? n1-standard-1
dchung-2rg5i2
```

Now you a list:

```shell
$ infrakit instance --name instance-gcp describe
ID                            	LOGICAL                       	TAGS
dchung-kw5i37                 	  -                           	infrakit-created=2017-04-05,infrakit-user=dchung,infrakit.scope=docker4x,startup-script=#!/bin/bash
sudo apt-get update -y
sudo apt-get install wget curl
wget -q0- https://get.docker.com | sh

```

Note the tags.. we can apply template to define the view on the tags.

```shell
$ infrakit instance --name instance-gcp describe --tags-view="{{ len . }}"
ID                            	LOGICAL                       	TAGS
dchung-kw5i37                 	  -                           	4
```

Also, we can query for details and print out the raw data, as YAML:

```shell
$ infrakit instance --name instance-gcp describe -pry
- ID: dchung-kw5i37
  LogicalID: null
  Properties:
    cpuPlatform: Intel Ivy Bridge
    creationTimestamp: 2017-04-05T00:30:08.433-07:00
    description: Some description
    disks:
    - autoDelete: true
      boot: true
      deviceName: persistent-disk-0
      interface: SCSI
      kind: compute#attachedDisk
      licenses:
      - https://www.googleapis.com/compute/v1/projects/ubuntu-os-cloud/global/licenses/ubuntu-1404-trusty
      mode: READ_WRITE
      source: https://www.googleapis.com/compute/v1/projects/docker4x/zones/us-central1-f/disks/dchung-kw5i37
      type: PERSISTENT
    id: "5569308151087965167"
    kind: compute#instance
    machineType: https://www.googleapis.com/compute/v1/projects/docker4x/zones/us-central1-f/machineTypes/n1-standard-1
    metadata:
      fingerprint: zMKHzh-yLow=
      items:
      - key: infrakit--scope
        value: docker4x
      - key: infrakit-created
        value: 2017-04-05
      - key: infrakit-user
        value: dchung
      - key: startup-script
        value: |
          #!/bin/bash
          sudo apt-get update -y
          sudo apt-get install wget curl
          wget -q0- https://get.docker.com | sh
      kind: compute#metadata
    name: dchung-kw5i37
    networkInterfaces:
    - accessConfigs:
      - kind: compute#accessConfig
        name: external-nat
        natIP: 104.154.99.160
        type: ONE_TO_ONE_NAT
      kind: compute#networkInterface
      name: nic0
      network: https://www.googleapis.com/compute/v1/projects/docker4x/global/networks/default
      networkIP: 10.128.0.3
      subnetwork: https://www.googleapis.com/compute/v1/projects/docker4x/regions/us-central1/subnetworks/default
    scheduling:
      automaticRestart: true
      onHostMaintenance: MIGRATE
    selfLink: https://www.googleapis.com/compute/v1/projects/docker4x/zones/us-central1-f/instances/dchung-kw5i37
    serviceAccounts:
    - email: 118031819273-compute@developer.gserviceaccount.com
      scopes:
      - https://www.googleapis.com/auth/cloudruntimeconfig
      - https://www.googleapis.com/auth/logging.write
    status: RUNNING
    tags:
      fingerprint: pDfT_HVxXHI=
      items:
      - dchung
    zone: https://www.googleapis.com/compute/v1/projects/docker4x/zones/us-central1-f
  Tags:
    infrakit-created: 2017-04-05
    infrakit-user: dchung
    infrakit.scope: docker4x
    startup-script: |
      #!/bin/bash
      sudo apt-get update -y
      sudo apt-get install wget curl
      wget -q0- https://get.docker.com | sh
```

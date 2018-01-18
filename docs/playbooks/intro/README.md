Quick Start
===========

## Install InfraKit

### Mac
If you don't have infrakit or go compiler installed locally, just

```shell

docker run --rm -v `pwd`:/build -e GOARCH=amd64 -e GOOS=darwin infrakit/installer build-infrakit
sudo cp ./infrakit /usr/local/bin
```
This will cross-compile the `infrakit` cli for Mac OSX.  For Linux, there's no need to set the `GOOS` and `GOARCH`
environment variables.

### Linux

```shell
$ docker run --rm -v `pwd`:/build -e GOARCH=amd64 -e GOOS=linux infrakit/installer build-infrakit
$ sudo cp ./infrakit /usr/local/bin/
```

## Set the `INFRAKIT_HOME` environment variable

In your terminal, or add this to your `.bash_profile`:

```shell
export INFRAKIT_HOME=~/.infrakit
```
If you're on the Mac, also

```shell
export INFRAKIT_HOST=localhost
```

## Add a Playbook

The `infrakit` cli is a dynamic command line tool.  You can add 'playbooks' which contain pre-built features.
This directory is in fact a playbook (see `index.ikb`).  To add this playbook, do this:

```shell

infrakit playbook add intro https://docker.github.io/infrakit/playbooks/intro/index.ikb
```

Verify that the playbook `intro` has been added:

```shell

infrakit playbook intro -h


___  ________   ________ ________  ________  ___  __    ___  _________
|\  \|\   ___  \|\  _____\\   __  \|\   __  \|\  \|\  \ |\  \|\___   ___\
\ \  \ \  \\ \  \ \  \__/\ \  \|\  \ \  \|\  \ \  \/  /|\ \  \|___ \  \_|
 \ \  \ \  \\ \  \ \   __\\ \   _  _\ \   __  \ \   ___  \ \  \   \ \  \
  \ \  \ \  \\ \  \ \  \_| \ \  \\  \\ \  \ \  \ \  \\ \  \ \  \   \ \  \
   \ \__\ \__\\ \__\ \__\   \ \__\\ _\\ \__\ \__\ \__\\ \__\ \__\   \ \__\
    \|__|\|__| \|__|\|__|    \|__|\|__|\|__|\|__|\|__| \|__|\|__|    \|__|


intro

Usage:
  infrakit playbook intro [command]

Available Commands:
  aws            aws
  darwin         darwin
  digitalocean   do
  events         events
  gcp            gcp
  start-infrakit start-infrakit
  stop-infrakit  stop-infrakit
```

## Prerequisites:

This tutorial uses Digitial Ocean.  Be sure you have the API token; you will be prompted to provide it
if you don't provide it in the command line flag.  You can also authenticate via the `doctl` tool and thus
have a `~/.config/doctl/config.yaml` file which has the access token.

Also, be sure to have upload a SSH key with Digital Ocean.  Creating instances requires specifying the SSH
key by name.


## Start up Infrakit controller daemons and plugins

The CLI will guide you through starting up infrakit controllers if command line flags were not given.
In the example below, we start up only the Digital Ocean plugin along with the rest of Infrakit controllers.
All the controllers and plugins are running as Docker containers:

```shell

$ infrakit playbook intro start-infrakit
Infrakit image? [infrakit/devbundle:dev]:
Infrakit port for remote access? [24864]:
What's the name of the project? [testproject]:
Start AWS plugin? [no]: n
Start GCP plugin? [no]: n
Start HYPERKIT plugin? [no]: n
Start Digital Ocean plugin? [no]: y
Access token? [8bf983553fe7a8001c4bf0a1f78f621f88e8344f5f03fcc9d67e25dfe4284a97]:
What's the DO plugin image? [infrakit/digitalocean:dev]:
Starting up infrakit base...  You can connect to it at infrakit -H localhost:24864
a2cdaf853e97c3650a6ed49aee9c215ab275600cdc4d0a1b71b69cfb6ecefe33
ab62ad58898d75711f7b72509102165926de1999ee79a7bb39a68a46bd75d779
31cfeff18ca6d25d5aed120ba24028729a27a72f8cb0bc7d8651f6a137ac2c2c
0f5e36fd945c332ab75630debf2b4564ab4546347e603f437547d2158cc33950
591a9dd4d92a8c40e581b6f30558f550c6d350e23a41696a14db153c4e96b05e
5819b2269742df7c23eecc9db26b443596277d1aad90a958145de977f89df23b
47b5b36ed6118832fc3e26e27b916a78f30b3e218e67b663708f1b2f097b7640
Starting DO plugin with image infrakit/digitalocean:dev, project testproject
6d1c76689845b866d16e84c6762e857f1055a676b1be6a1a2fb106e66d6da17d
```

Because Infrakit in the case is running as Docker containers, on the Mac,
make sure you have the Docker4Mac host set up. To verify:

```shell
infrakit remote ls
HOST                          	URL LIST
docker4mac                    	localhost:24864
```

If it's not there, then you can set it:

```shell
infrakit remote add docker4mac localhost:24864
```

You can add different remotes via this command. Right now we only need this
on the Mac for Docker4Mac.

To connect to this target, set your environment variable `INFRAKIT_HOST`:

```shell
export INFRAKIT_HOST=docker4mac
```

Now your CLI is pointing to the Docker4Mac remote.  To change it, e.g. if you have
a remote AWS cluster, just add that as a remote and change the `INFRAKIT_HOST` environment variable
to the name of the remote.


## Provision an Instance

There's a `digitalocean` sub-command, for Digital Ocean.  It has the following command available:

```shell
$ infrakit playbook intro digitialocean -h


___  ________   ________ ________  ________  ___  __    ___  _________
|\  \|\   ___  \|\  _____\\   __  \|\   __  \|\  \|\  \ |\  \|\___   ___\
\ \  \ \  \\ \  \ \  \__/\ \  \|\  \ \  \|\  \ \  \/  /|\ \  \|___ \  \_|
 \ \  \ \  \\ \  \ \   __\\ \   _  _\ \   __  \ \   ___  \ \  \   \ \  \
  \ \  \ \  \\ \  \ \  \_| \ \  \\  \\ \  \ \  \ \  \\ \  \ \  \   \ \  \
   \ \__\ \__\\ \__\ \__\   \ \__\\ _\\ \__\ \__\ \__\\ \__\ \__\   \ \__\
    \|__|\|__| \|__|\|__|    \|__|\|__|\|__|\|__|\|__| \|__|\|__|    \|__|


do

Usage:
  infrakit playbook intro digitalocean [command]

Available Commands:
  provision-instance provision-instance
  scale-group        scale-group
  start-daemon       start-daemon
  start-plugin       start-plugin
```

Now provision.  Use flags or follow the prompts:

```shell

$ infrakit playbook intro digitialocean provision-instance
Owner? [davidchung]:
Region? [sfo1]:
Image to boot? [ubuntu-16-10-x64]:
Instance size? [1gb]:
Private IP address (IPv4)? :
SSH key to use? : infrakit
47604302
```

You can verify the instance you've created on Digital Ocean:

```shell
$ infrakit instance --name instance-digitalocean describe -pry
- ID: "47604302"
  LogicalID: null
  Properties:
    created_at: 2017-05-02T04:01:18Z
    disk: 30
    id: 47604302
    image:
      created_at: 2017-04-26T20:24:29Z
      distribution: Ubuntu
      id: 24439774
      min_disk_size: 20
      name: 16.10 x64
      public: true
      regions:
      - nyc1
      - sfo1
      - nyc2
      - ams2
      - sgp1
      - lon1
      - nyc3
      - ams3
      - fra1
      - tor1
      - sfo2
      - blr1
      slug: ubuntu-16-10-x64
      type: snapshot
    memory: 1024
    name: davidchung-6o4i6o
    networks:
      v4:
      - gateway: 162.243.152.1
        ip_address: 162.243.155.7
        netmask: 255.255.248.0
        type: public
    region:
      available: true
      features:
      - private_networking
      - backups
      - ipv6
      - metadata
      - install_agent
      name: San Francisco 1
      sizes:
      - 512mb
      - 1gb
      - 2gb
      - 4gb
      - 8gb
      - 16gb
      - 32gb
      - 48gb
      - 64gb
      slug: sfo1
    size:
      available: true
      disk: 30
      memory: 1024
      price_hourly: 0.01488
      price_monthly: 10
      regions:
      - ams1
      - ams2
      - ams3
      - blr1
      - fra1
      - lon1
      - nyc1
      - nyc2
      - nyc3
      - sfo1
      - sfo2
      - sgp1
      - tor1
      slug: 1gb
      transfer: 2
      vcpus: 1
    size_slug: 1gb
    status: active
    tags:
    - davidchung
    - infrakit::user:davidchung
    - infrakit-do-version:1
    - infrakit::created:2017-05-01
    - infrakit::scope:testproject
    vcpus: 1
    volume_ids: []
  Tags:
    davidchung: ""
    infrakit-do-version: "1"
    infrakit.created: 2017-05-01
    infrakit.scope: testproject
    infrakit.user: davidchung
```

## Destroy the instance

```shell

$ infrakit instance --name instance-digitalocean describe
ID                            	LOGICAL                       	TAGS
47604302                      	  -                           	davidchung=,infrakit-do-version=1,infrakit.created=2017-05-01,infrakit.scope=testproject,infrakit.user=davidchung
$ infrakit instance --name instance-digitalocean destroy 47604302
destroyed 47604302

```

## Manage a Scale Group

The Digital Ocean playbook `digitalocean` also has a function to create a scale group of nodes:

```shell

$ infrakit playbook intro digitialocean scale-group -h

scale-group

Usage:
  infrakit playbook intro digitialocean scale-group [flags]

Flags:
      --group-name string      Name of group
      --image-id string        Image  ID
      --instance-size string   Instance size
      --project string         Project name
      --region string          DO region
      --size int               Size of the group
      --ssh-key string         SSH key to use

```

You can use flags or follow along:

```shell

$ infrakit playbook intro digitialocean scale-group
How many nodes? [2]: 3
Name of the group? [mygroup]:
Region? [sfo1]:
Image to boot? [ubuntu-16-10-x64]:
Instance size? [1gb]:
SSH key to use? : infrakit
Project? [myproject]: tutorial
Group mygroup with plugin group plan: Managing 3 instances
```

Now you can check on them

```shell
infrakit group ls
ID
mygroup
Davids-MacBook-Pro-4:~ davidchung$ infrakit group describe mygroup
ID                            	LOGICAL                       	TAGS
47605058                      	  -                           	infrakit-do-version=1,infrakit.config.hash=aclsbhfuk3pwoh42y55xqd7ulan6pfq4,infrakit.group=mygroup,infrakit.scope=testproject,mygroup=,project=tutorial
47605059                      	  -                           	infrakit-do-version=1,infrakit.config.hash=aclsbhfuk3pwoh42y55xqd7ulan6pfq4,infrakit.group=mygroup,infrakit.scope=testproject,mygroup=,project=tutorial
47605060                      	  -                           	infrakit-do-version=1,infrakit.config.hash=aclsbhfuk3pwoh42y55xqd7ulan6pfq4,infrakit.group=mygroup,infrakit.scope=testproject,mygroup=,project=tutorial
```

You can kill a node and see the group recover:

```shell
$ infrakit instance-digitalocean destroy 47605058
destroyed 47605058
$ infrakit group describe mygroup
ID                            	LOGICAL                       	TAGS
47605059                      	  -                           	infrakit-do-version=1,infrakit.config.hash=aclsbhfuk3pwoh42y55xqd7ulan6pfq4,infrakit.group=mygroup,infrakit.scope=testproject,mygroup=,project=tutorial
47605060                      	  -                           	infrakit-do-version=1,infrakit.config.hash=aclsbhfuk3pwoh42y55xqd7ulan6pfq4,infrakit.group=mygroup,infrakit.scope=testproject,mygroup=,project=tutorial

# .... after 10-20 seconds

$ infrakit group describe mygroup
ID                            	LOGICAL                       	TAGS
47605059                      	  -                           	infrakit-do-version=1,infrakit.config.hash=aclsbhfuk3pwoh42y55xqd7ulan6pfq4,infrakit.group=mygroup,infrakit.scope=testproject,mygroup=,project=tutorial
47605060                      	  -                           	infrakit-do-version=1,infrakit.config.hash=aclsbhfuk3pwoh42y55xqd7ulan6pfq4,infrakit.group=mygroup,infrakit.scope=testproject,mygroup=,project=tutorial
47606009                      	  -                           	infrakit-do-version=1,infrakit.config.hash=aclsbhfuk3pwoh42y55xqd7ulan6pfq4,infrakit.group=mygroup,infrakit.scope=testproject,mygroup=,project=tutorial
```

## Scale up/down the group

First let's get the group's specification from the manager:

```shell
$ infrakit manager inspect -y
- Plugin: group
  Properties:
    ID: mygroup
    Properties:
      Allocation:
        LogicalIDs: null
        Size: 3
      Flavor:
        Plugin: flavor-vanilla
        Properties:
          Init:
          - apt-get update -y
          - apt-get install wget curl
          - wget -qO- https://get.docker.com | sh
          Tags:
            project: tutorial
      Instance:
        Plugin: instance-digitalocean
        Properties:
          Image:
            Slug: ubuntu-16-10-x64
          NamePrefix: mygroup
          PrivateNetworking: false
          Region: sfo1
          SSHKeyNames:
          - infrakit
          Size: 1gb
          Tags:
          - mygroup
```

Save this output as file. Edit it and then commit it:

```shell

$ infrakit manager inspect -y > mygroup.yml

```
Edit the file to change the `Size` to 4:

```
$ vi mygroup.yml
```

Now commit this new specification:

```shell
$ infrakit manager commit -y - < mygroup.yml
Group mygroup with plugin group plan: Adding 1 instances to increase the group size to 4
```

After a bit, check the group:

```shell
$ infrakit group describe mygroup
ID                            	LOGICAL                       	TAGS
47605060                      	  -                           	infrakit-do-version=1,infrakit.config.hash=aclsbhfuk3pwoh42y55xqd7ulan6pfq4,infrakit.group=mygroup,infrakit.scope=testproject,mygroup=,project=tutorial
47606009                      	  -                           	infrakit-do-version=1,infrakit.config.hash=aclsbhfuk3pwoh42y55xqd7ulan6pfq4,infrakit.group=mygroup,infrakit.scope=testproject,mygroup=,project=tutorial
47606041                      	  -                           	infrakit-do-version=1,infrakit.config.hash=aclsbhfuk3pwoh42y55xqd7ulan6pfq4,infrakit.group=mygroup,infrakit.scope=testproject,mygroup=,project=tutorial
47606318                      	  -                           	infrakit-do-version=1,infrakit.config.hash=aclsbhfuk3pwoh42y55xqd7ulan6pfq4,infrakit.group=mygroup,infrakit.scope=testproject,mygroup=,project=tutorial
```

## Destroy the Group

```shell
$ infrakit group destroy mygroup
destroy mygroup initiated
```

And then check:

```shell
$ infrakit group describe mygroup
CRIT[05-01|21:32:31] error executing                          module=main cmd=infrakit err="Group 'mygroup' is not being watched" fn=main.main
Group 'mygroup' is not being watched
```
Now query directly the instance plugin for *any* nodes:

```shell
$ infrakit instance-digitalocean describe
ID                            	LOGICAL                       	TAGS
```

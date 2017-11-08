Running InfraKit on GCP
=======================

The `infrakit` cli is a dynamic command line tool.  You can add 'playbooks' which contain pre-built features.
This directory is in fact a playbook (see `index.ikb`).  To add this playbook, do this:

```shell

infrakit playbook add gcp https://docker.github.io/infrakit/playbooks/intro/gcp/index.ikb
```

Verify that the playbook `gcp` has been added:

```shell

infrakit playbook gcp -h


___  ________   ________ ________  ________  ___  __    ___  _________
|\  \|\   ___  \|\  _____\\   __  \|\   __  \|\  \|\  \ |\  \|\___   ___\
\ \  \ \  \\ \  \ \  \__/\ \  \|\  \ \  \|\  \ \  \/  /|\ \  \|___ \  \_|
 \ \  \ \  \\ \  \ \   __\\ \   _  _\ \   __  \ \   ___  \ \  \   \ \  \
  \ \  \ \  \\ \  \ \  \_| \ \  \\  \\ \  \ \  \ \  \\ \  \ \  \   \ \  \
   \ \__\ \__\\ \__\ \__\   \ \__\\ _\\ \__\ \__\ \__\\ \__\ \__\   \ \__\
    \|__|\|__| \|__|\|__|    \|__|\|__|\|__|\|__|\|__| \|__|\|__|    \|__|


gcp

Usage:
  infrakit playbook gcp [command]

Available Commands:
  provision-instance provision-instance
  start-daemon       start-daemon

...

Use "infrakit playbook gcp [command] --help" for more information about a command.
```

## Prerequisites:

This tutorial uses Google Cloud Platform.  Be sure you have the API token; you will be prompted to provide it
if you don't provide it in the command line flag.  You can also authenticate via the `gcloud` tool and thus
have a `~/.config/gcloud/application_default_credentials.json` file which has the access token.

## Start up Infrakit controller daemons and plugins

The CLI will guide you through starting up infrakit controllers if command line flags were not given.
In the example below, we start up only the Google Cloud Platform plugin along with the rest of Infrakit controllers.
All the controllers and plugins are running as Docker containers:

```shell

$ infrakit playbook gcp start-daemon
Credentials JSON path? [/home/frenchben/.config/gcloud/application_default_credentials.json]:
What's the zone? : us-west1-a
What's the name of the project? : my-gcp-project
Starting daemon
Tailing log

```

## Provision an Instance

To provision an instance on GCP, use the flags or follow the prompts:

```shell

$ infrakit playbook gcp provision-instance
Owner? [frenchben]:
Image to boot? [https://www.googleapis.com/compute/v1/projects/ubuntu-os-cloud/global/images/ubuntu-1404-trusty-v20161205]:
Machine type? [n1-standard-1]:
Private IP address (IPv4)? [10.128.0.10]:
Disk size in GB? [100]:

```
Quick Start
===========

## Install InfraKit

### Mac
If you don't have infrakit or go compiler installed locally, just

```shell

docker run --rm -v `pwd`:/build infrakit/installer build-infrakit darwin
sudo cp ./infrakit /usr/local/bin
```
This will cross-compile the `infrakit` cli for Mac OSX.  For Linux, there's no need to set the `GOOS` and `GOARCH`
environment variables.

### Linux

```shell
$ docker run --rm -v `pwd`:/build infrakit/installer build-infrakit linux
$ sudo cp ./infrakit /usr/local/bin/
```

## Set the `INFRAKIT_HOME` environment variable

In your terminal, or add this to your `.bash_profile`:

```shell
export INFRAKIT_HOME=~/.infrakit
```

## Add a Playbook

The `infrakit` cli is a dynamic command line tool.  You can add 'playbooks' which contain pre-built features.
There are two playbooks you can try:

  + Intro
  + Linuxkit POC


### LinuxKit + Infrakit POC
This is a simple playbook that combines LinuxKit with InfraKit into a single workflow for building
custom OS images and deploying them onto your local Mac VM instances as well as servers on GCP or Packet.net.
For GCP and Packet, you will need proper billing account set up to launch virtual machines.

```shell

infrakit playbook add linuxkit https://docker.github.io/infrakit/playbooks/linuxkit/index.yml
```

If you have cloned this repo, you can speed things up using the `file://` url:

```shell

infrakit playbook add linuxkit file:///Users/changeme/github.com/docker/infrakit/docs/playbooks/linuxkit/index.yml
```




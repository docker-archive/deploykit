LinuxKit Playbook
=================

This is a demo playbook for working with LinuxKit.

## How to Add

1. Make sure you have the `infrakit` CLI installed --
  + Either build from source (see [tutorial](../../tutorial.md)), or
  + Use built containers and cross-compile locally -- see [Quick Start](../README.md).

2. Add via the `playbook` command:

```shell
$ infrakit playbook add linuxkit https://docker.github.io/infrakit/playbooks/linuxkit/index.yml
$ infrakit playbook ls
PLAYBOOK                      	URL
linuxkit                      	https://docker.github.io/infrakit/playbooks/linuxkit/index.yml
```

The URL `https://docker.github.io/infrakit/playbooks/linuxkit/index.yml` is for the file `index.yml`
that is being served by github pages.  If you cloned the repo, you can speed up things by referencing
a local file via the `file://` scheme in the URL.


Now verify:
```shell
$ infrakit playbook linuxkit -h


linuxkit

Usage:
  infrakit playbook linuxkit [command]

Available Commands:
  demo-sshd        demo-sshd
  install-hyperkit install-hyperkit
  install-moby     install-moby
  run-gcp          run-gcp
  run-hyperkit     run-hyperkit
  run-packet       run-packet
  start            start
  stop             stop
```

This playbook assumes you're running on a Mac, using Docker for Mac as the runtime for running the InfraKit plugins.
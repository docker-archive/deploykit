LinuxKit `sshd` Example
=======================

This example shows how to build a LinuxKit image containing just a simple sshd.

The file `sshd.yml` defines the components inside the image.  Instead of a standard
LinuxKit image yml, it is actually an InfraKit template that is rendered *before* the
moby tool is invoked to build the actual OS image.

## Build Image

The command `build-image` will collect user input such as the public key location
and use that to generate the final input to `moby`.

## Platforms in this example

There are two subcommands for booting up your vm instance on different platforms:

  + hyperkit -- this is for running vm instances on your Mac.
  + packet -- this is for running vm instances on Packet.net (bare-metal hosts)

For each platform there are subcommands available:

  + run-instance   - creates and boots up a single instance
  + scale-group    - starts up a group of instances
  + destroy-all    - destroy all the instances
  + list-instances - list all the instances


## Run Instance

Using the `hyperkit` subcommand (does not require billing accounts / signup on providers),
you can create a single or a cluster of instances after you run the `build-image`.

The command `... hyperkit run-instance` will use hyperkit plugin to create a single guest vm
that boots from the image you built with `build-image`.

After the instance is running, you can check via

```
infrakit instance-hyperkit describe
```

### Scale Group

There are other commands to try -- like starting up a cluster:

```
infrakit playbook linuxkit demo-sshd hyperkit scale-group
```

A quick `describe` will show the new instances:

```
infrakit instance-hyperkit describe
```

### Change the Public Key

Try running `build-image` again, and this time use a different public key.
Then run

```
infrakit playbook linuxkit demo-sshd hyperkit scale-group
```

After you answer all the questions, you should notice that Infrakit will attempt a rolling update.
This is because the OS image has changed -- Infrakit has detected the change and initiates a rolling update.

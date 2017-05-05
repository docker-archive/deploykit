LinuxKit `sshd` Example
=======================

This example shows how to build a LinuxKit image containing just a simple sshd.

The file `sshd.yml` defines the components inside the image.  Instead of a standard
LinuxKit image yml, it is actually an InfraKit template that is rendered *before* the
moby tool is invoked to build the actual OS image.

## Build Image

The command `build-image` will collect user input such as the public key location
and use that to generate the final input to `moby`.

## Run Instance

The command `run-instance` will use hyperkit plugin to create a single guest vm
that boots from the image you built with `build-image`.

After the instance is running, you can check via

```
infrakit instance --name instance-hyperkit describe
```

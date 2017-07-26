InfraKit Instance Plugin - VMware vCenter
===============================

An [InfraKit](https://github.com/docker/infrakit/blob/master/README.md) plugin
for creating and managing Virtual Machine resources inside of [VMware vCenter](https://www.vmware.com/products/vcenter-server.html). This plugin communicates directly with the VMware vCenter Web SDK through the use of the VMware [govmomi](https://github.com/vmware/govmomi) library.

## Building and Running

To build the vsphere plugin, run `make binaries`. The plugin will be located at `./build/infrakist-instance-vsphere`.

The *minimum* requirement to start the plugin is the URL+Credentials of a VMware vCenter server in the form of: `https://``Username`:`Password`@`vCenterAddress/sdk`
This can either be passed to through the environment variable `VCURL` or with the flag `--url=`.

#### NOTE:
For **Testing** purposes the plugin can also be started with `--ignoreOnDestroy=true`, when this is set InfraKit will not actually delete VMs but only "label" them as deleted.

## Plugin Design

The vCenter Plugin makes use of two methods to track InfraKit instances:

- vCenter Folders, that follow the naming convention of the InfraKit Group
- vSphere VM annotation notes that are applied to each new created VM, that track both the group attachement and the configuration. The configuration tracking allows InfraKit to know when a VM will need rebuilding from a new config.

When the plugin needs to identify VMs that it is managing it will:

1. Retrieve VMs from the folder that matches the InfraKit group name.
2. It will then iterate through those VMs looking at the annotation to identify VMs that InfraKit manages in that folder.
3. It will then only return to InfraKit the VMs that are both in the folder and have the InfraKit group ID attached to them, all other VMs are ignored by InfraKit.

This allows end-users to drag runnning VMs from the folder for other use cases (such as debugging and VM inspection). When that VM is removed, InfraKit will notice that the Virtual Machine allocation for the folder is incorrect and rebalance the group.

## Supported Features

- Multiple DataCentre support (for people with multiple DCs in a vCenter cluster)
- Use of Folders and VM annotations to track InfraKit VMs
- Provisioning, Scaling and Destroying
- Cloning of Virtual Machine Templates, to be used as the base image for new instances
- Building of new VM instances around LinuxKist `.iso` files that are uploades through the `$ linuxkit push vcenter <args> file.iso` command.
- Capability to identify VM guest IP address settings throught the `infrakit group describe <groupID>`
- Can be used with a single vSphere Host, however Folders aren't supported so all VMs will end up as part of the root structure.

## Plugin Usage

### Additional flags / environment variables when starting the plugin

- `--datacenter` or `VCDATACENTER` = name of a Datacenter within vCenter
- `--network` or env `VCNETWORK` = Name of network e.g. `VM Network`
- `--datastore` or `VCDATASTORE` = Default Datastore used for new VM Instances
- `--hostname` or `VCHOST` = Name of the main vSphere host that will be used

**Note**: These may be depricated as they should be `commited` through the infrakit cli instead.

### Example JSON with a Template
```
      "Properties": {
         "Datastore" : "datastore1",
         "Network" : "VM Network",
         "Hostname" : "localhost.localdomain",
         "Template" : "CentOS7 - NGINX",
         "VMPrefix" : "lk", 
         "PowerOn" : true
      }
```

### Example JSON with LinuxKit

```
      "Properties": {
         "Datastore" : "datastore1",
         "Network" : "VM Network",
         "Hostname" : "localhost.localdomain",
         "ISOPath" : "linuxkit/linuxkit.iso",
         "CPUs" : "1",
         "MEM" : "512",
         "VMPrefix" : "lk", 
         "PowerOn" : true
      }
```


## Outstanding Issues

- On initial creation of VMs, there can be a warning as the plugin can try to create the same folder in quick succession to hold the new VMs. **However**, these errors can be ignored as the plugin will safely create the new VM instances once the folder has been created.
- The Plugin will login to vCenter when first started and will continue to remain logged in **only** whilst it is monitoring created instances. If all instances are destroyed and the plugin isn't monitoring vCenter then the login will eventually timeout and the plugin will need restarting.

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


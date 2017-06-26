InfraKit Instance Plugin - VMware vCenter/vSphere
===============================

This plugin allows Docker InfraKit to automate the provisioning of Virtual Machine instances. 

### Current State
- InfraKit will provision from LinuxKit `.iso` files that can be uploaded using the `$ linuxkit push vcenter <args> file.iso` command.
- Provisioning, Scaling and Destroying
- An InfraKit Group will create a VMware vCenter folder that will hold all VM instances that are tied to that group.
- On vSphere it will use the `annotation` field to hold the InfraKit group and config SHA.
- Environment variables can be used with the plugin for VMware vCenter configuration:
	- `VCURL` = URL for VMware vCenter/vSphere `https://User:Pass@X.X.X.X/sdk`
	- `VCNETWORK` = Name of network e.g. `VM Network`
	- `VCDATASTORE` = Default Datastore used for new VM Instances
	- `VCHOST` = Name of the main vSphere host that will be used.
- The following JSON parameters provide VM Instance configuration:

```
      "Properties": {
         "Datastore" : "datastore1",
         "Network" : "VM Network",
         "Hostname" : "localhost.localdomain",
         "isoPath" : "linuxkit/linuxkit.iso",
         "CPUs" : "1",
         "MEM" : "512",
         "vmPrefix" : "lk", 
         "powerOn" : true
      }

```
- Moved to `pkg/providers/vsphere`


## To Do
- Investigate error messages, when using vSphere instead of VMware vCenter
- VMware Template support `.vmtx`
- `Makefile` needs updating




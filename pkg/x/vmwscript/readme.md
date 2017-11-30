# InfraKit VMwScript [experimental]

A tool to make use of the VMware APIs to automate the provisioning of virtual machines.

There are two provided examples that detail the usage, one will update a CentOS (latest release) VMware
template to support the new release of Docker-CE. The second will create a swarm master and add two 
swarm workers to the cluster. These two examples cover a lot of the usage and command structure for the 
build plans.

**Windows support** Still to be tested, but no reason it shouldn't work

## Building
Clone the InfraKit repository and then:

```
$ git clone https://github.com/infrakit
$ cd infrakit
$ make get-tools
$ make build/infrakit
```

(Instructions are on the [main page](http://github.com/docker/infrakit)) 



## Usage

VMware vCenter configuration details can be passed in three ways, either in the JSON/Environment variables or through flags to the executable.

```
$ ./build/infrakit x vmwscript

Usage:
  infrakit x vmwscript deployment.json [flags]

Flags:
      --datacenter string     The name of the Datacenter to host the VM [REQD]
      --datastore string      The name of the DataStore to host the VM [REQD]
      --hostname string       The server that will run the VM [REQD]
      --network string        The network label the VM will use [REQD]
      --templatePass string   The password for the specified user inside the VM template
      --templateUser string   A created user inside of the VM template
      --vcurl string          VMware vCenter URL, format https://user:pass@address/sdk [REQD]
      --vmcommand string      A command passed as a string to be executed on the virtual machine specified with [--vmname]
      --vmname string         The name of an existing virtual machine to run a command against
      --vmsudouser string     A sudo user that the command will be executed
```

### Environment variables

Below are the environment variables that can be used to authenticate when using the InfraKit vmwscript utility.

```
export INFRAKIT_VSPHERE_VCDATACENTER="Datacenter"
export INFRAKIT_VSPHERE_VCDATASTORE="datastore"
export INFRAKIT_VSPHERE_VCHOST="vsphere01.lab"
export INFRAKIT_VSPHERE_VCNETWORK="Internal Network (NAT)"
export INFRAKIT_VSPHERE_VCURL="https://user@vsphere.local:pass@vCenter.lab/sdk"
```

### Standalone usage

The `vmwscript` utility has the capability to run commands against an already created and running virtual machine.

**Example**: restarting Apache to pick up a new configuration

`./infrakit x vmwscript --vmname=web01 --vmsudouser=root --vmcommand="systemctl restart httpd"`

### Deployment files

To cover how the Deployment files work we will use sections from the example files for [building templates, or swarm clusters](./examples/). 

**Header**

```
{
    "label":"Docker-CE-on-CentOS",
    "version":"0.1",
    "vmconfig" : {
        "datacentre" : "",
        "datastore":"",
        "network" : "",
        "host" : "",
        "guestCredentials" : {
            "guestUser" : "root",
            "guestPass" :"password"
        }
    },
    
```
 
The `label` is what vmwscript will use when it logs out what the deployment plan is when the configuration is ran and is mainly for tracking/aesthetics as is `version`. The `vmconfig` block *can* be used to hold the configuration details that is used to authenticate with a VMware vCenter server, however flags or environment variables can also be used. The `guestCredentials` block is **[REQUIRED]** as these are the credentials that the vmtools will attempt to run all commands inside the virtual Machine as.

**Deployment Tasks**

```
    "deployment": [
        {"name": "Docker Template",
         "note": "Build new template for CentOS",
         "task":{
            "inputTemplate": "Centos7-Template",
            "outputName": "DockerTemplate",
            "outputType": "Template",
            "import":"",
            "networkConfig":{
                    "distro":"centos",
                    "device":"ens192",
                    "address":"10.0.0.101/24",
                    "gateway":"10.0.0.1",
                    "dns":"8.8.8.8",
                    "hostname":"manager01.local"
                },
```

The `deployment` is an array of deployments that can take place, each JSON object in that array will be a full set of deployment tasks that will be defined with a `name` and a `note`. 

Each task has some defined inputs and outputs:

- `inputTemplate` - Specifies a VMware template to use
- `outputName` - The name of the VM output that will be created
- `outputType` - Either `Template` or `VM`
- `import` - **UNUSED** possibly for linking deployment plans together
- `commands` - A JSON array of commands detailed below
- `networkConfig` - A static configuration to be applied per each host

**Network Config**

- `distro` - Currently only `centos` has been created
- `device` - This is the device that is used by Network Manager (typically `ens160` or `ens192`)
- `address` - This should be the in the `address`/`subnet` format
- `gateway` - The IP address of the gateway being used
- `dns` - This can either be a single or multiple DNS servers e.g. `8.8.8.8 8.8.4.4`
- `hostname` - This will set the instance to have a particular hostname that persists through reboots.

The virtual machine will *REBOOT* once the networking changes have been applied.

**Execute a simple command**

This command will pull a *worker* token from a swarm cluster to allow other hosts to join, this token is stored in a file in `/tmp`

```
                   {
                       "type":"execute",                    
                       "note":"Backing up swarm key for other nodes",            
                       "cmd":"/usr/bin/docker swarm join-token worker | grep SWMTKN > /tmp/swm.tkn",
                       "sudoUser":"root"
                   },              
```

**Download output and store as a vmwscript key**

This command will download the key that we stored in the previous command, and hold it in memory under the vmwscript key `jointoken`.

```
                   {
                       "type":"download",
                       "filePath":"/tmp/swm.tkn",
                       "resultKey":"jointoken",
                       "delAfterDownload": true
                   }
```

**Execute a command from a key**

In another set of deployment tasks, we will execute a command that we stored in the previous command.

```
                   {
                       "type":"execute",                    
                       "note":"Join Swarm",
                       "execKey":"jointoken",
                       "sudoUser":"root"
                   }
```

### Summary of output from building a three node swarm cluster


```
$ ./build/infrakit x vmwscript ./pkg/x/vmwscript/examples/swarm.json 
INFO[11-08|12:01:35] Finished parsing [Docker-CE-on-CentOS], [3] tasks will be deployed module=x/vmwscript fn=github.com/docker/infrakit/pkg/x/vmwscript.OpenFile
INFO[11-08|12:01:35] Starting VMwScript engine                module=cli/x fn=github.com/docker/infrakit/cmd/infrakit/x.vmwscriptCommand.func1
INFO[11-08|12:01:35] Beginning Task                           module=x/vmwscript Task:="Swarm Manager" Notes="Build Swarm leader from Template" fn=github.com/docker/infrakit/pkg/x/vmwscript.RunTasks
INFO[11-08|12:01:36] Cloning a New Virtual Machine            module=x/vmwscript fn=github.com/docker/infrakit/pkg/x/vmwscript.Provision
INFO[11-08|12:02:28] Modifying Networking backend             module=x/vmwscript fn=github.com/docker/infrakit/pkg/x/vmwscript.Provision
INFO[11-08|12:02:28] Waiting for VMware Tools and Network connectivity... module=x/vmwscript fn=github.com/docker/infrakit/pkg/x/vmwscript.Provision
INFO[11-08|12:03:14] New Virtual Machine has started its networking module=x/vmwscript IP Address=10.0.0.3 fn=github.com/docker/infrakit/pkg/x/vmwscript.Provision
INFO[11-08|12:03:14] 3 commands will be executed.             module=x/vmwscript fn=github.com/docker/infrakit/pkg/x/vmwscript.runCommands
INFO[11-08|12:03:14] Task: Initialise Docker Swarm            module=x/vmwscript fn=github.com/docker/infrakit/pkg/x/vmwscript.runCommands
INFO[11-08|12:03:15] Watching process [1515] cmd ["/bin/sudo" -n -u root /usr/bin/docker swarm init] module=x/vmwscript fn=github.com/docker/infrakit/pkg/x/vmwscript.watchPid
Task completed in 1 Seconds

{...}

INFO[11-08|12:06:34] Task: Join Swarm                         module=x/vmwscript fn=github.com/docker/infrakit/pkg/x/vmwscript.runCommands
INFO[11-08|12:06:34] Executing command from key [jointoken]   module=x/vmwscript fn=github.com/docker/infrakit/pkg/x/vmwscript.runCommands
INFO[11-08|12:06:35] Watching process [1520] cmd ["/bin/sudo" -n -u root     docker swarm join --token SWMTKN-1-4o4styrpxv9uappv46w6u6m0b8amw2kklt6ntk0x41l2yjs7qn-9sgwfr98av1pfl28imytg0etv 10.0.0.3:2377
] module=x/vmwscript fn=github.com/docker/infrakit/pkg/x/vmwscript.watchPid
Task completed in 0 Seconds
INFO[11-08|12:06:36] VMwScript has completed succesfully      module=cli/x fn=github.com/docker/infrakit/cmd/infrakit/x.vmwscriptCommand.func1

```

# InfraKit.RackHD

**Docker for Private Clouds**

The first and only open source toolkit for creating and managing declarative, self-healing, and platform-agnostic infrastructure in your data center. 


[InfraKit](https://github.com/docker/infrakit) plugins for creating and managing resources in [RackHD](https://github.com/RackHD/RackHD).

![logo](img/rackhd-infrakit-logo.png "Logo")

## Instance plugin

An InfraKit instance plugin is provided, which runs RackHD workflows to provision compute nodes.

### Building and running

To build the RackHD Instance plugin, run `make binaries`.  The plugin binary will be located at
`./build/infrakit-instance-rackhd`.

### Example

TODO

## Licensing
infrakit.rackhd is freely distributed under the [MIT License](http://codedellemc.github.io/sampledocs/LICENSE "LICENSE"). See LICENSE for details.

##Support
Please file bugs and issues on the Github issues page for this project. This is to help keep track and document everything related to this repo. For general discussions and further support you can join the [{code} by Dell EMC Community slack team](http://community.codedellemc.com/) and join the **#rackhd** channel. The code and documentation are released with no warranties or SLAs and are intended to be supported through a community driven process.

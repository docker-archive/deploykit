# HPE OneView Infrakit Instance Plugin

This is a Docker InfraKit Instance Plugin that is designed to automate the provisioning of "Instances" through HPE OneView (Servers currently). This plugin is driven by configuration that is passed in to Docker InfraKit typically using the group plugin to manage numerous instances or numerous groups of instances (multi-tenancy of sorts).


## Architecture Overview

The Instance plugin will take the configuration that is described in the group-default JSON configuration and commit the instance code to the plugin itself. The plugin then will communicate directly with OneView to assess the state of defined instances and act accordingly by (**creating**, **growing**, **healing**, **removing** or **destroying**) the instances. If numerous group configurations are commited, then the oneview plugin will manage all instances and will differentiate between instances and which group they belong too.

![OneView Architecture](http://thebsdbox.co.uk/wp-content/uploads/2016/11/InfraKit-Instance-oneview.jpeg)

### Plugin socket
Ideally the various Docker InfraKit plugins are meant to be started inside of containers, to expose communication between the various plugins (which takes place over UNIX sockets) the path where the sockets are created should be mapped with the `-v` docker flag. Like the Docker InfraKit standard all plugin sockets are created in `$HOME/.infrakit/plugins`.

### State data

In order to manage expected state with actual state the plugin will use Group tags and the hash of the configuration to manage the state. The plugin also implements a simple state machine to watch for profiles that are being created but not yet listed by the API.

The **Group Tags** live inside of HPE OneView instances are used in order to allow the plugin to determine which state file (detailed above) an instances is described within. 

## Using the plugin

**Starting**

The plugin can be started quite simply by running `./infrakit plugin start instance-oneview` which will start the plugin with all of the defaults for **socket** and **state** files located within the `~/.infrakit` directories. Once the plugin is up and running it can be discovered through the InfraKit cli through the command `infrakit plugin ls`. 

**Configuration**

To pass authentication credentials to the HPE OneView plugin, it should be started with a number of environment variables:

`INFRAKIT_ONEVIEW_OVURL` = IP address of OneView

`INFRAKIT_ONEVIEW_OVUSER` = Username to connect to OneView

`INFRAKIT_ONEVIEW_OVPASS` = Password to authenticate to OneView

`INFRAKIT_ONEVIEW_OVAPI` = Specify an API version *[optional, defaults to 300]*


As with all InfraKit plugins, the group plugin will define the *"amount"* of instances that need to be provisioned the instance plugins. The group plugin will then pass the instance configuration to the plugin as many times as needed. The main points of note in the instance configuration:

`TemplateName : string` = **[required]** This has to match (case sensitive) a pre-created OneView template

```
"Instance" : {
      "Plugin" : "instance-oneview",
      "Properties" : {
        "Note" : "Generic OneView configuration",
        "TemplateName" : "Docker-Gen8-Template",
      }
    },
```


## NEXT STEPS


* TBD

# Copyright and license

Copyright © 2017 Docker, Inc. All rights reserved. Released under the Apache 2.0
license. See [LICENSE](LICENSE) for the full license text.

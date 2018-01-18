InfraKit Instance Plugin - Docker
=================================

[InfraKit](https://github.com/docker/infrakit) plugins for creating and managing Docker containers.

## Instance plugin

The InfraKit instance plugin creates and monitors Docker containers.

### Example

Based on the [default](https://github.com/docker/infrakit/tree/master/cmd/group) Group
plugin:
```console
$ build/infrakit-group-default
INFO[0000] Starting discovery
INFO[0000] Starting plugin
INFO[0000] Starting
INFO[0000] Listening on: unix:///run/infrakit/plugins/group.sock
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/group.sock err= <nil>
```

the [Vanilla](https://github.com/docker/infrakit/tree/master/examples/flavor/vanilla) Flavor plugin:
```console
$ build/infrakit-flavor-vanilla
INFO[0000] Starting plugin
INFO[0000] Listening on: unix:///run/infrakit/plugins/flavor-vanilla.sock
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/flavor-vanilla.sock err= <nil>
```

and the Docker Instance plugin:

```console
$ build/infrakit-instance-docker
INFO[0000] PID file at unix:///run/infrakit/plugins/instance-docker.pid
INFO[0000] Server waiting at unix:///run/infrakit/plugins/instance-docker
```

We will use a basic configuration that creates a single instance:
```console
$ cat << EOF > docker-vanilla.json
{
  "ID": "docker-example",
  "Properties": {
    "Allocation": {
      "Size": 1
    },
    "Instance": {
      "Plugin": "instance-docker",
      "Properties": {
        "Config": {
          "Image": "httpd:2.2"
        },
        "HostConfig": {
          "AutoRemove": true
        },
        "Tags": {
          "Name": "infrakit-example"
        }
      }
    },
    "Flavor": {
      "Plugin": "flavor-vanilla",
      "Properties": {
        "Init": [
          "sh -c \"echo 'Hello, World!' > /hello\""
        ]
      }
    }
  }
}
EOF
```

For the structure of `Config` `HostConfig`, see the plugin properties definition below.

Finally, instruct the Group plugin to `commit` the group:
```console
$ build/infrakit group commit docker-vanilla.json
Committed docker-example: Managing 1 instances
```

Additionally, the CLI will report the newly-created instance:
```console
$ build/infrakit group describe docker-example
ID                             	LOGICAL                        	TAGS
90e6f3de4918                   	elusive_leaky                  	Name=infrakit-example,infrakit.config.hash=dUBtWGmkptbGg29ecBgv1VJYzys=,infrakit.group=docker-example
```

Retrieve the name of the container and connect to it with an exec

```console
$ docker exec -ti elusive_leaky cat /hello
Hello, World!
```

### Plugin properties

The plugin expects properties in the following format:
```json
{
  "Tags": {
  },
  "Config": {
  },
  "HostConfig": {
  },
  "NetworkAttachments": [
  ]
}
```

The `Tags` property is a string-string mapping of labels to apply to all Docker containers that are created.
`Config` follows the structure of the type by the same name in the
[Docker go SDK](https://github.com/docker/docker/blob/master/api/types/container/config.go).
`HostConfig` follows the structure of the type by the same name in the
[Docker go SDK](https://github.com/docker/docker/blob/master/api/types/container/host_config.go).
`NetworkAttachments` is an array of [NetworkResource](https://github.com/docker/docker/blob/master/api/types/types.go).

### LogicalID

To take advantage of the Docker networking DNS, the InfraKit logicalID is mapped to the Docker container hostname (and not its IP).
The plugin is compatible with both allocation methods, logical IDs (cattles) or group size (pets).

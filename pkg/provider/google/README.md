# InfraKit.GCP

[![CircleCI](https://circleci.com/gh/docker/infrakit/pkg/provider/google.svg?style=shield&circle-token=28d281a3090845d1c42c36298ff878a7c9bb6ffa)](https://circleci.com/gh/docker/infrakit/pkg/provider/google)

[InfraKit](https://github.com/docker/infrakit) plugins for creating and managing resources in Google Cloud Platform.

## Instance plugin

An InfraKit instance plugin which creates Google Compute Engine instances.

### Building

To build the instance plugin, run `make binaries`.  The plugin will then show up as part of the main infrakit CLI
`build/infrakit plugin start google`.

### Running

```bash
INFRAKIT_GOOGLE_PROJECT=my-project INFRAKIT_GOOGLE_ZONE=us-west1 build/infrakit plugin start google
```

#### Project and zone selection

Google Cloud project and zone can be passed on the command line with `INFRAKIT_GOOGLE_PROJECT`
and `INFRAKIT_GOOGLE_ZONE` env variables. In case a value is not provided, the plugin will fallback to:
 + Querying the [Metadata server][metadata] when running on GCE
 + `CLOUDSDK_CORE_PROJECT` and `CLOUDSDK_CORE_ZONE` environment variables

[metadata]: https://cloud.google.com/compute/docs/storing-retrieving-metadata

#### Pets versus Cattle

Groups defined with an `Allocation/Size` will create 'cattle' instances that
are fully disposable. When an instance is deleted, a completely new one will be
recreated. this new instance will have a different name and a different disk.

Groups defined with an `Allocation/LogicalIDs` will create 'pet' instances.
When an instance is deleted, a new one will be created, with the same name. It
will also try to reuse the disk named after the instance if it was not deleted
too.

### Example configuration

To continue with an example, we will use the [default](https://github.com/docker/infrakit/tree/master/cmd/group) Group
plugin:

```bash
$ build/infrakit plugin start group
INFO[11-06|16:58:10] config                                   module=cli/plugin url= fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:58:10] Launching                                module=cli/plugin kind=group name=group-stateless fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:58:10] Starting plugin                          module=core/launch executor=inproc key=group name=group-stateless exec=inproc fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-06|16:58:10] Object is an event producer              module=rpc/server object=&{keyed:0xc4201862f8} discover=/Users/infrakit/.infrakit/plugins/group-stateless fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[11-06|16:58:10] Listening                                module=rpc/server discover=/Users/infrakit/.infrakit/plugins/group-stateless fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[11-06|16:58:10] Waiting for startup                      module=core/launch key=group name=group-stateless config="{\n\"Kind\": \"group\",\n\"Options\": {\n\"PollInterval\": \"10s\",\n\"MaxParallelNum\": 0,\n\"PollIntervalGroupSpec\": \"10s\",\n\"PollIntervalGroupDetail\": \"10s\"\n}\n}" as=group-stateless fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-06|16:58:10] Done waiting on plugin starts            module=cli/plugin fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:58:10] PID file created                         module=run path=/Users/infrakit/.infrakit/plugins/group-stateless.pid fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-06|16:58:10] Server started                           module=run discovery=/Users/infrakit/.infrakit/plugins/group-stateless fn=github.com/docker/infrakit/pkg/run.run.func1
```

and the [Vanilla](https://github.com/docker/infrakit/tree/master/pkg/example/flavor/vanilla) Flavor plugin:

```bash
$ build/infrakit plugin start vanilla
INFO[11-06|16:59:01] config                                   module=cli/plugin url= fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:59:01] Launching                                module=cli/plugin kind=vanilla name=vanilla fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:59:01] Starting plugin                          module=core/launch executor=inproc key=vanilla name=vanilla exec=inproc fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-06|16:59:01] Listening                                module=rpc/server discover=/Users/infrakit/.infrakit/plugins/vanilla fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[11-06|16:59:01] Waiting for startup                      module=core/launch key=vanilla name=vanilla config="{\n\"Kind\": \"vanilla\",\n\"Options\": {\n\"DelimLeft\": \"\",\n\"DelimRight\": \"\",\n\"MultiPass\": true,\n\"CacheDir\": \"\"\n}\n}" as=vanilla fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-06|16:59:01] Done waiting on plugin starts            module=cli/plugin fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:59:01] PID file created                         module=run path=/Users/infrakit/.infrakit/plugins/vanilla.pid fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-06|16:59:01] Server started                           module=run discovery=/Users/infrakit/.infrakit/plugins/vanilla fn=github.com/docker/infrakit/pkg/run.run.func1
```

We will use a basic configuration that creates a single instance:

```bash
$ cat << EOF > gcp-vanilla.json
{
  "ID": "gcp-example",
  "Properties": {
    "Allocation": {
      "Size": 1
    },
    "Instance": {
      "Plugin": "google/compute",
      "Properties": {
        "NamePrefix": "test",
        "Description": "Test of Google infrakit",
        "Network": "default",
        "Tags": ["tag1", "tag2"],
        "MachineType": "n1-standard-1",
        "Disks":[{
            "Boot": true,
            "SizeGb": 60,
            "Image": "https://www.googleapis.com/compute/v1/projects/ubuntu-os-cloud/global/images/ubuntu-1404-trusty-v20161205",
            "Type": "pd-standard"
        }],
        "Scopes": [
          "https://www.googleapis.com/auth/cloudruntimeconfig",
          "https://www.googleapis.com/auth/logging.write"
        ]
      }
    },
    "Flavor": {
      "Plugin": "vanilla",
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

Finally, instruct the Group plugin to start watching the group:

```bash
$ build/infrakit group-stateless commit gcp-vanilla.json
```

### Permissions
If running on a GCP instance, please make sure that the instance has service accounts enabled:
https://cloud.google.com/compute/docs/access/create-enable-service-accounts-for-instances

You can check that the instance has the correct permissions via: `gcloud compute instances list`

## Group plugin

An InfraKit group plugin which wraps Google Compute Engine's managed instance
groups.

The CLI will report the newly-created instance from the above example, via:

```bash
$ build/infrakit group-stateless inspect gcp-example
```

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

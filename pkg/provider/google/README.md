# InfraKit.GCP

[![CircleCI](https://circleci.com/gh/docker/infrakit.gcp.svg?style=shield&circle-token=28d281a3090845d1c42c36298ff878a7c9bb6ffa)](https://circleci.com/gh/docker/infrakit.gcp)

[InfraKit](https://github.com/docker/infrakit) plugins for creating and managing resources in Google Cloud Platform.

## Instance plugin

An InfraKit instance plugin which creates Google Compute Engine instances.

### Building

To build the instance plugin, run `make binaries`.  The plugin binary will be located at
`./build/infrakit-instance-gcp`.

### Running

```bash
${PATH_TO_INFRAKIT}/infrakit-flavor-vanilla
${PATH_TO_INFRAKIT}/infrakit-group-default
./build/infrakit-instance-gcp --project=[GCP_PROJECT] --zone=[GCP_ZONE]

${PATH_TO_INFRAKIT}/infrakit group commit gcp-example-1.json
```

#### Project and zone selection

Google Cloud project and zone can be passed on the command line with `--project`
and `--zone`. In case a value is not provided, the plugin will fallback to:
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

```json
{
  "ID": "gcp-example-1",
  "Properties": {
    "Allocation": {
      "Size": 1
    },
    "Instance": {
      "Plugin": "instance-gcp",
      "Properties": {
        "NamePrefix": "test",
        "Description": "Test of GCP infrakit",
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
      "Plugin": "flavor-vanilla",
      "Properties": {
        "Init": [
          "sh -c \"echo 'Hello, World!' > /hello\""
        ]
      }
    }
  }
}
```

## Group plugin

An InfraKit group plugin which wraps Google Compute Engine's managed instance
groups.

### Building

To build the group plugin, run `make binaries`.  The plugin binary will be located at
`./build/infrakit-group-gcp`.

### Running

```bash
${PATH_TO_INFRAKIT}/infrakit-flavor-vanilla
./build/infrakit-instance-gcp --project=[GCP_PROJECT] --zone=[GCP_ZONE] --name=group

${PATH_TO_INFRAKIT}/infrakit group commit gcp-example-2.json
```

#### Project and zone selection

Works the same as the instance plugin.

#### Pets versus Cattle

This plugin supports only pets via `Allocation/Size`. It doesn't support
`Allocation/LogicalIDs`.
This plugin doesn't need an instance plugin since instances are managed directly
by GCP.

### Example configuration

```json
{
  "ID": "gcp-example-2",
  "Properties": {
    "Allocation": {
      "Size": 2
    },
    "Instance": {
      "Properties": {
        "Description": "Test of GCP infrakit",
        "Network": "default",
        "Tags": ["tag1", "tag2"],
        "MachineType": "n1-standard-1",
        "Disks":[{
            "Boot": true,
            "SizeGb": 60,
            "Image": "https://www.googleapis.com/compute/v1/projects/ubuntu-os-cloud/global/images/ubuntu-1404-trusty-v20161205",
            "Type": "pd-standard",
            "AutoDelete": false,
            "ReuseExisting": true
        }],
        "Scopes": [
          "https://www.googleapis.com/auth/cloudruntimeconfig",
          "https://www.googleapis.com/auth/logging.write"
        ]
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

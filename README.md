# InfraKit.GCP

[![CircleCI](https://circleci.com/gh/docker/infrakit.gcp.svg?style=shield&circle-token=28d281a3090845d1c42c36298ff878a7c9bb6ffa)](https://circleci.com/gh/docker/infrakit.gcp)

[InfraKit](https://github.com/docker/infrakit) plugins for creating and managing resources in Google Cloud Platform.

## Instance plugin

An InfraKit instance plugin is provided, which creates Google Compute Engine instances.

### Building and running

To build the GCP Instance plugin, run `make binaries`.  The plugin binary will be located at
`./build/infrakit-instance-gcp`.

### Example configuration

```
$ cat gcp-example.json
{
  "ID": "gcp-example",
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
        "DiskSizeMb": 60,
        "DiskImage": "https://www.googleapis.com/compute/v1/projects/ubuntu-os-cloud/global/images/ubuntu-1404-trusty-v20161205",
        "DiskType": "pd-standard",
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

`infrakit group commit gcp-example.json`


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

# InfraKit.AWS

[![CircleCI](https://circleci.com/gh/docker/infrakit.aws.svg?style=shield&circle-token=e74dcf8c25027948307a7618041e1d1997ded50a)](https://circleci.com/gh/docker/infrakit.aws)

[InfraKit](https://github.com/docker/infrakit) plugins for creating and managing resources in Amazon AWS.

## Instance plugin

An InfraKit instance plugin is provided, creates Amazon EC2 instances.

### Building and running

To build the AWS Instance plugin, run `make binaries`.  The plugin binary will be located at
`./bin/infrakit-instance-aws`.

### Plugin properties

The plugin expects properties in the following format:
```json
{
  "Tags": {
  },
  "RunInstancesInput": {
  }
}
```

The `Tags` property is a string-string mapping of EC2 instance tags to include on all instances that are created.
`RunInstancesInput` follows the structure of the type by the same name in the
[AWS go SDK](http://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#RunInstancesInput).


#### AWS API Credentials

The plugin can use API credentials from several sources.
- config file: See [AWS docs](http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html#cli-environment)
- EC2 instance metadata See [AWS docs](http://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2.html)

Additional credentials sources are supported, but are not generally recommended as they are less secure:
- command line arguments: `--session-token`, or  `--access-key-id` and `--secret-access-key`
- environment variables: See [AWS docs](http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html#cli-environment)


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

Example Playbook for AWS
========================

This playbook contains example of working with AWS.  There are commands for
starting up infrakit (which assumes you have used AWS CLI on your local computer and
the `.aws/credentials` file exists) and commands for spinning up an on-demand or
spot instance configured with Docker, Git and Go compiler.

## Adding this playbook

Adding files locally:

```
infrakit playbook add aws file://$(pwd)/index.yml
```

Adding the playbook from Github:

```
infrakit playbook add aws https://raw.githubusercontent.com/docker/infrakit/master/examples/playbooks/aws/index.yml
```

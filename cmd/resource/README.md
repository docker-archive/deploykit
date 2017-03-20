InfraKit Resource Plugin
========================

The Resource plugin works in conjuction with Instance plugins to manage collections of
interdependent, named resources.  Dependencies are expressed with the `resource "name"` template
function, the output of which is the ID of the named resource.

## Running

Begin by building plugin [binaries](../../README.md#binaries).

The plugin may be started without any arguments and will default to using unix socket in
`~/.infrakit/plugins` for communications with the CLI and other plugins:

```shell
$ build/infrakit-resource
INFO[0000] Listening at: ~/.infrakit/plugins/resource
```

## Working with the Resource Plugin

Start the resource plugin as show above.

Start the `instance-file` plugin:
```shell
$ mkdir -p instances
$ build/infrakit-instance-file --dir instances
INFO[0000] Listening at: ~/.infrakit/plugins/instance-file
INFO[0000] PID file at ~/.infrakit/plugins/instance-file.pid
```

Save the following in a file named resources.json.
```json
{
  "ID": "Fancy Resources",
  "Properties": {
    "Resources": {
      "A": {
        "Plugin": "instance-file",
        "Properties": {}
      },
      "B": {
        "Plugin": "instance-file",
        "Properties": {
          "Note": "Depends on {{ resource `A` }}"
        }
      }
    }
  }
}
```

Commit resources.json:
```shell
$ build/infrakit resource commit file:$PWD/resources.json
INFO[0000] Reading template from file:~/resources.json
Committed Fancy Resources:
Provisioned A (ID instance-9021654729849995625)
Provisioned B (ID instance-2978607024592968517)
```

Verify the presence of `A`'s ID in `B`:

```shell
$ jq .Spec.Properties.Note instances/instance-2978607024592968517
"Depends on instance-9021654729849995625"
```

InfraKit Instance Plugin - File
===============================

A [reference](/README.md#reference-implementations) implementation of an Instance Plugin that can accept any
configuration and writes the configuration to disk as `provision`.  It is useful for testing and debugging.

## Building

Begin by building plugin [binaries](/README.md#binaries).

## Usage

The plugin can be started without any arguments and will default to using unix socket in
`~/.infrakit/plugins` for communications with the CLI and other plugins:

```shell
$ INFRAKIT_INSTANCE_FILE_DIR=./test build/infrakit plugin start file
INFO[0000] Listening at: ~/.infrakit/plugins/file
```

The environment variable `INFRAKIT_INSTANCE_FILE_DIR` sets the directory
used by this plugin instance.  This starts the plugin starts up the
plugin listening at socket file `file`.

You can give the another plugin instance a different name:
```shell
$ INFRAKIT_INSTANCE_FILE_DIR=./test2 build/infrakit plugn start file:another
INFO[0000] Listening at: ~/.infrakit/plugins/another
```

Be sure to verify that the plugin is [discoverable](/cmd/infrakit/README.md#list-plugins).

Note that there should be two file instance plugins running now with different names
(`file`, and `another`).

See the [CLI Doc](/cmd/infrakit/README.md) for details on accessing the instance plugin via CLI.

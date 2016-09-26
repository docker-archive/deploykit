InfraKit Instance Plugin - File
===============================

This is a simple Instance Plugin that can accept any configuration and writes the configuration
to disk as `provision`.  It is useful for testing and debugging.

## Building

When you do `make -k all` in the top level directory, the CLI binary will be built and can be
found as `./infrakit/cli` from the project's top level directory.

## Usage

```
$ ./infrakit/file -h
File instance plugin

Usage:
  ./infrakit/file [flags]
  ./infrakit/file [command]

Available Commands:
  version     print build version information

Flags:
      --dir string      Dir for storing the files (default "/var/folders/rq/g0cj3y7n511c10p2kbz5q33w0000gn/T/")
      --listen string   listen address (unix or tcp) for the control endpoint (default "unix:///run/infrakit/plugins/instance-file.sock")
      --log int         Logging level. 0 is least verbose. Max is 5 (default 4)

Use "./infrakit/file [command] --help" for more information about a command.
```

The plugin can be started without any arguments and will default to using unix socket in
`/run/infrakit/plugins` for communications with the CLI and other plugins:

```
$ ./infrakit/file --dir=./test
INFO[0000] Starting plugin
INFO[0000] Listening on: unix:///run/infrakit/plugins/instance-file.sock
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/instance-file.sock err= <nil>
```

This starts the plugin using `./test` as directory and `instance-file` as name.

You can give the another plugin instance a different name via the `listen` flag:
```
$ ./infrakit/file --listen=unix:///run/infrakit/plugins/another-file.sock --dir=./test
INFO[0000] Starting plugin
INFO[0000] Listening on: unix:///run/infrakit/plugins/another-file.sock
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/another-file.sock err= <nil>
```

Using the CLI, it you can see

```
$ ./infrakit/cli plugin ls
Plugins:
NAME                	LISTEN
group               	unix:///run/infrakit/plugins/group.sock
instance-file       	unix:///run/infrakit/plugins/instance-file.sock
another-file        	unix:///run/infrakit/plugins/another-file.sock
flavor-zookeeper    	unix:///run/infrakit/plugins/flavor-zookeeper.sock
```
Note that there are two file instance plugins running now with different names.


### Default Directory for Plugin Discovery

All InfraKit plugins will by default open the unix socket located at `/run/infrakit/plugins`.
Make sure this directory exists on your host:

```
mkdir -p /run/infrakit/plugins
chmod 777 /run/infrakit/plugins
```

See the [CLI Doc](../../../cli/README.md) for details on accessing the instance plugin via CLI.

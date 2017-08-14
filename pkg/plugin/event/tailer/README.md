Tailer
======

This plugin can tail multiple files and generate events via the events SPI so that the client
can follow log files remotely.

You can use `infrakit plugin start` to start the plugin.  Without a configuration JSON for
defining rules on starting up plugins, you can do:

```
# Assuming at the top of the project directory

# Run a file logger
pkg/plugin/event/tailer/test1.sh &  # will generate a file called test1.log in the same diretory

INFRAKIT_EVENT_TAILER_PATH=${PWD}/pkg/plugin/event/tailer/test1.log infrakit plugin start tailer:mylogfile
```

This is the simplest case of using the tailer.  Using the `infrakit` CLI, you will see

```
$ infrakit


infrakit command line interface

Usage:
  infrakit [command]

Available Commands:
  event       Access event exposed by infrakit plugins
  manager     Access the manager
  metadata    Access metadata exposed by infrakit plugins
  mylogfile   Access plugin mylogfile which implements Event/0.1.0,Metadata/0.1.0  <----- This is new
  playbook    Manage playbooks
  plugin      Manage plugins
  remote      Manage remotes
```

Now you can see the topics:

```
$ infrakit mylogfile event ls -al
total 1:
Users/.../pkg/plugin/event/tailer/test1.log
```

And you can tail the file

```
$ infrakit mylogfile event tail Users/.../pkg/plugin/event/tailer/test1.log
INFO[0000] Connecting to broker url= unix://mylogfile topic= Users/davidchung/project3/src/github.com/docker/infrakit/pkg/plugin/event/tailer/test1.log opts= {/Users/davidchung/.infrakit/plugins /events}
Mon Aug 14 01:16:42 PDT 2017 -- 16675
Mon Aug 14 01:16:43 PDT 2017 -- 18597
Mon Aug 14 01:16:44 PDT 2017 -- 21251
```

For more complex configuration, you can use a config JSON file (see [plugins.json](./plugins.json)).
In this example there are two keys `applogs` and `syslogs`, and each rule uses the `tailer` kind to tail
at least one file.  To start both you can simply

```
infrakit --log 5 plugin start --config-url file://${PWD}/pkg/plugin/event/tailer/plugins.json syslogs applogs
```

Note the args `syslogs` and `applogs` are keys in the plugins.json file.  Once this starts, you can see
there are two event plugins visible. One with the name `applogs` and the other, `syslogs`.

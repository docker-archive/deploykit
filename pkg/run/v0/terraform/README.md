How to work with the terraform kind
===================================

The file `terraform.go` here is like a 'main'.  Instead of creating complex command line CLI options,
you'd simply add define the data structure you need in the `Options` type and register a default value
with Infrakit's plugin launcher framework (see line 36 of `terraform.go`):

```
func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}
```

Now you need to create a `plugins.json` file and use that when you use `infrakit plugin start`.
This file will have the actual config parameters you need to start things up.

This format actually supports a variety of plugin startup mechanism (eg. exec to shell, to docker, etc.)
but the one we care about for best UX is the `inproc` which essentially embeds all the important plugins
in the same binary and will run in the same infrakit daemon process.  The `inproc` launch system uses
the go packages and registration mechanism mentioned above (in the `pkg/run/v0` and all the subpackages
below it).

The block here will set up the input for a key `terraform` which uses `inproc` subsystem
to launch the `terraform` kind:

```
    {
        "Key" : "terraform",
        "Launch" : {
            "inproc": {
		"Kind" : "terraform",
		"Options" : {
		    "Dir": "{{ env `INFRAKIT_HOME` }}/terraform",
		    "Standalone": true,
		    "NewOption" : "hello world"
		}
	    }
        }
    }
```

This configuration file is used by the `infrakit plugin start` command.  It has a usage like this:

```
infrakit plugin start [--config-url <url>] <plugin_spec> [ <plugin_spec> ... ]
```
where `--config-url` will point to the `plugins.json` file and
`<plugin_spec>` is `<key>[:<socke_file>[=<exec>]]`

For example:

```
infrakit plugin start terraform
```
is equal to
```
infrakit plugin start terraform:terraform=inproc
```

where the launch system will look for `Key` of `terraform`, and use the `inproc` launcher which uses the
embedded `terraform` Kind.  That then will look for the `Options` field and provide that to the code
that is in `terraform.go`.

Once you have the new fields defined and updated your `plugins.json` file to reflect these new changes,
you have to provide this `plugins.json` to complement/override a set of defaults in the system:

```
build/infrakit plugin --config-url file://$(pwd)/pkg/run/v0/terraform/plugins.json start terraform
INFO[09-12|02:49:51] Launching                                module=cli/plugin kind=terraform name=terraform fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[09-12|02:49:51] Starting plugin                          module=core/launch executor=inproc key=terraform name=terraform exec=inproc fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[09-12|02:49:51] No group spec URL specified for import   module=run/v0/terraform fn=github.com/docker/infrakit/pkg/run/v0/terraform.parseInstanceSpecFromGroup
INFO[09-12|02:49:51] NewOptions                               module=run/v0/terraform value="hello world" Dir=/Users/davidchung/.infrakit/terraform fn=github.com/docker/infrakit/pkg/run/v0/terraform.Run
INFO[09-12|02:49:51] Listening                                module=rpc/server discover=/Users/davidchung/.infrakit/plugins/terraform fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[09-12|02:49:51] Waiting for startup                      module=core/launch key=terraform name=terraform config="{\n\t\t\"Kind\" : \"terraform\",\n\t\t\"Options\" : {\n\t\t    \"Dir\": \"/Users/davidchung/.infrakit/terraform\",\n\t\t    \"Standalone\": true,\n\t\t    \"NewOption\" : \"hello world\"\n\t\t}\n\t    }" as=terraform fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[09-12|02:49:51] Done waiting on plugin starts            module=cli/plugin fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[0000] PID file at /Users/davidchung/.infrakit/plugins/terraform.pid
INFO[0000] Server waiting at /Users/davidchung/.infrakit/plugins/terraform
Empty or non-existent state file.

Refresh will do nothing. Refresh does not error or return an erroneous
exit status because many automation scripts use refresh, plan, then apply
and may not have a state file yet for the first run.


```

There -- your plugin just came up with customized fields and is now running inside the same process
as the rest of infrakit. Note that the value `hello world` for has been loaded and printed out in `INFO`.

A couple of notes:

  1. The `plugins.json` will be evaluated as a template.  This means you can use template functions
  like `env` or even `source`/`include` to bring in other complex configurations.
  2. The `Options` block in the schema actually corresponds to the `Options` in the global Spec
  (see pkg/types/Spec).  The design here allows you to start up the plugin before the global Spec schema
  is in place, and when that arrives, you'd simply move this block into the global schema so that a single
  specs file will contain all the information necessary about how your infrastructure, as well as the settings
  for the plugins (hence Options and *not* Properties).

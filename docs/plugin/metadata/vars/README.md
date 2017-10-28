Vars Plugin
===========

## Design

The `vars` plugin implements the [`metadata.Updatable`](../pkg/spi/metadata) SPI.  It supports

  + Reading of metadata values
  + Updating of metadata values

The base Metadata SPI looks like this:

```
// Plugin is the interface for metadata-related operations.
type Plugin interface {

	// List returns a list of *child nodes* given a path, which is specified as a slice
	List(path types.Path) (child []string, err error)

	// Get retrieves the value at path given.
	Get(path types.Path) (value *types.Any, err error)
}
```

The `Updatable` SPI adds two methods for changes and commit:

```
// Updatable is the interface for updating metadata
type Updatable interface {

	// Plugin - embeds a readonly plugin interface
	Plugin

	// Changes sends a batch of changes and gets in return a proposed view of configuration and a cas hash.
	Changes(changes []Change) (original, proposed *types.Any, cas string, err error)

	// Commit asks the plugin to commit the proposed view with the cas.  The cas is used for
	// optimistic concurrency control.
	Commit(proposed *types.Any, cas string) error
}
```

where `Change` captures atom of change:

```
// Change is an update to the metadata / config
type Change struct {
	Path  types.Path
	Value *types.Any
}
```

Updating metadata entries involve creating a set of Changes and then sending it to the plugin to get
a proposal, a hash, and a view of the current dataset in its entirety.  This is followed by a commit
which takes the returned proposal view and the hash.  If the data has been updated before the commit
is issued, this commit will fail because the hash value will be different now than the one returned
via the `Changes` call.  This is how the plugin handles optimistic concurrency.

The plugin takes a template URL as a way to initialize itself with some data.  This is useful for cases
where there's already a set of parameters in an JSON that's in version control, and you can use that
as an initial value.  The URL is set as an [`Option` attribute](./pkg/run/v0/vars/vars.go) so it can
be specified in the `plugins.json` passed to `infrakit plugin start`.  You can also use the environment
variable `INFRAKIT_VARS_TEMPLATE` as a way to set it.

Corresponding to the SPI methods, there are new commands / verbs as `metadata`:

  + `keys` lists the metadata paths
  + `cat` reads metadata values
  + `change` updates metadata values, provided the plugin implements the updatable SPI (not readonly)

You can try it out using the `vars` kind:

```shell
 INFRAKIT_VARS_TEMPLATE=file://$(pwd)/docs/plugin/metadata/vars/example.json infrakit plugin start vars
```

We start up the plugin using `example.json` here as initial values:
```
{{ var `cluster/user/name` `user` }}
{{ var `zones/east/cidr` `10.20.100.100/24` }}
{{ var `zones/west/cidr` `10.20.100.200/24` }}

{
    "cluster" : {
	"size" : 5,
	"name" : "test"
    },
    "shell" : {{ env `SHELL` }}
}
```

Note that this file is itself a template.  You can 'export' parameter values via the `var` function, as
well as, creating the actual JSON object in this document.

Now in another terminal session, you should see `vars` show up as a subcommand in `infrakit`

```shell
$ infrakit -h


infrakit command line interface

Usage:
  infrakit [command]

Available Commands:
  manager     Access the manager
  playbook    Manage playbooks
  plugin      Manage plugins
  remote      Manage remotes
  template    Render an infrakit template at given url.  If url is '-', read from stdin
  up          Up everything
  util        Utilities
  vars        Access object vars which implements Updatable/0.1.0
  version     Print build version information
  x           Experimental features
```

Getting help:

```shell
$ infrakit vars -h


Access metadata of vars

Usage:
  infrakit vars [command]

Available Commands:
  cat         Get metadata entry by path
  change      Update metadata where args are key=value pairs and keys are within namespace of the plugin.
  keys        List metadata
```

### Listing, Reading

Listing metadata values:

```shell
$ infrakit vars keys -al
total 6:
cluster/name
cluster/size
cluster/user/name
shell
zones/east/cidr
zones/west/cidr
```

or

```shell
$ infrakit vars keys -al zones
total 2:
east/cidr
west/cidr
```

Reading a value using `cat`:

```shell
$ infrakit vars cat zones/east/cidr
10.20.100.100/24
```

Complex values:

```shell
$ infrakit vars cat zones
{"east":{"cidr":"10.20.100.100/24"},"west":{"cidr":"10.20.100.200/24"}}
```

You can also use the `metadata` template function and evaluate an inline template:

```shell
$ infrakit template 'str://The CIDR is {{ metadata `vars/zones/east/cidr`}}!'
The CIDR is 10.20.100.100/24!
```

Formatting it as YAML, if the value at a given path is actually a struct/object:

```shell
$ infrakit template 'str://{{ metadata `vars/zones` | yamlEncode }}'
east:
  cidr: 10.20.100.100/24
west:
  cidr: 10.20.100.200/24
```

### Updating

In the `infrakit` CLI, the `Changes` + `Commit` steps have been combined into a single `change`
verb with `-c` for commit.

The `change` verb is followed by a list of `name=value` pairs which are committed togeter as one
unit.  The `change` verb either prints the proposal or prints the proposal and commits if `-c` is set.

No changes means getting a dump of the entire plugin's metadata as a document:

```shell
$ infrakit vars change
Proposing 0 changes, hash=f34a016c93733536ebd5de6e3e7aa87c
{
  "cluster": {
    "name": "test",
    "size": 5,
    "user": {
      "name": "user"
    }
  },
  "shell": "/bin/bash",
  "zones": {
    "east": {
      "cidr": "10.20.100.100/24"
    },
    "west": {
      "cidr": "10.20.100.200/24"
    }
  }
}
```

Updating multiple values:

```shell
$ infrakit vars change cluster/name=hello shell=/bin/zsh zones/east/cidr=10.20.100/16
Proposing 3 changes, hash=0d2e7576bafc24c7f07839f77fad6952
{
  "cluster": {
    "name": "thestllo",
    "size": 5,
    "user": {
      "name": "user"
    }
  },
  "shell": "/bin/bazsh",
  "zones": {
    "east": {
      "cidr": "10.20.100.100/2416"
    },
    "west": {
      "cidr": "10.20.100.200/24"
    }
  }
}
```

Not shown above, your terminal should show color differences of the change.  Using the `-c` option will
commit the change (which has the hash `0d2e7576bafc24c7f07839f77fad6952`):

```shell
$ infrakit vars change cluster/name=hello shell=/bin/zsh zones/east/cidr=10.20.100/16 -c
Committing 3 changes, hash=0d2e7576bafc24c7f07839f77fad6952
{
  "cluster": {
    "name": "thestllo",
    "size": 5,
    "user": {
      "name": "user"
    }
  },
  "shell": "/bin/bazsh",
  "zones": {
    "east": {
      "cidr": "10.20.100.100/2416"
    },
    "west": {
      "cidr": "10.20.100.200/24"
    }
  }
}
```

Verify:

```shell
$ infrakit vars cat cluster/name
hello
$ infrakit vars cat zones/east/cidr
10.20.100/16
```

You can also add new values / structs:

```shell
$ infrakit vars change this/is/a/new/struct='{ "message":"i am here"}' -c
Committing 1 changes, hash=1c5bb84ad728337950127a3a4710509d
{
  "cluster": {
    "name": "hello",
    "size": 5,
    "user": {
      "name": "user"
    }
  },
  "shell": "/bin/zsh",
  "this": {
    "is": {
      "a": {
        "new": {
          "struct": {
            "message": "i am here"
          }
        }
      }
    }
  },
  "zones": {
    "east": {
      "cidr": "10.20.100/16"
    },
    "west": {
      "cidr": "10.20.100.200/24"
    }
  }
}
```

Verify:

```shell
$ infrakit vars cat this/is/a/new/struct/message
i am here
```

## Template integration

As mentioned earlier, metadata operations are integrated with templates so you
can write templates that also access the variables stored in the vars plugin.

As an example:

```shell
$infrakit plugin start vars  # starts up only the vars 
```

In another shell:

```shell
$ infrakit vars change
Proposing 0 changes, hash=c53f4ebe9b2a50bc2b52fd88a5d503e1
{}
```

Currently there's no data stored.

Adding some values:

```shell
$ infrakit vars change cluster/name=foo cluster/workers/size=10 -c
Committing 2 changes, hash=ad861ded7a0742f9662c2b78354c89f6
{
  "cluster": {
    "name": "foo",
    "workers": {
      "size": 10
    }
  }
}
```

Now querying:

```shell
$ infrakit vars keys -al
total 2:
cluster/name
cluster/workers/size
$ infrakit vars cat cluster/workers/size
10
```
Or use template to read.

```shell
$ infrakit template -f 'str://{{ metadata `vars/cluster/workers/size`}}'
10
```

Using template to update:

```shell
$ infrakit template -f 'str://{{ metadata `vars/cluster/workers/size` 1000 }}'
~/project3/src/github.com/docker/infrakit$ infrakit vars change
Proposing 0 changes, hash=8aff842d0f19e571e5c1e5810d562bb1
{
  "cluster": {
    "name": "foo",
    "workers": {
      "size": 1000
    }
  }
}
$ infrakit vars cat cluster/workers/size
1000
```

Sometimes, you want to have a single namespace between metadata and template
variables to simplify writing templates.  The `var` template function, in
correct contexts, can attempt to retrieve the value at the given path via accessing
the path as though it's hosted in a metadata plugin, if the path cannot be resolved
to an in-scope template variable:

```shell
$ infrakit template 'str://{{ var `vars/cluster/workers/size`}}'
1000
```
which is the same as

```shell
$ infrakit template 'str://{{ metadata `vars/cluster/workers/size`}}'
1000
```

This works even when a template is multi-pass: if a path cannot be resolved
from in-scope template variables, then metadata plugins are tested.  Of course,
the path has to also make sense in that the path can resolve to a running metadata
plugin.

If somewhere else in your template, a `var` function is used to set a value and
that value comes into scope of the template, that value will takes precedence:

```shell
$ infrakit template 'str://{{ var `vars/cluster/workers/size` 4000 }}{{ var `vars/cluster/workers/size`}}'
4000
```
This should not be surprising, as we are setting the `vars/cluster/workers/size`
template variable to `4000`.  The next read will return that value and not the
metadata stored in vars:

```shell
$ infrakit template 'str://{{ var `vars/cluster/workers/size`}}'
1000
```

The value is back to 1000 because the second command line invocation is a
totally different template context (ie a different shell process altogether).
This makes it possible to implement a user override, while the backing
metadata can be loaded from a JSON serving as default values.
This also means that mutation of the value at a given path must be performed
explicitly via the `metadata` template function (or via the `change -c` CLI) to
affect durability across multiple template evaluations.

## TODO - Durability of Changes

The metadata / updatable plugin is one of the key patterns provided by Infrakit. The base implementation
of this plugin does not store state, like all of the plugins (e.g. Instance and Group controllers).
This means all the `change` you apply will be gone when the plugin process exits.
While this is useful for the case of storing some kind of secrets (which is prompted from user and then
set in memory), there are many cases where we want to persist the user's changes.

In keeping with the design philosophy of layering and composition, the `vars` plugin which does not store state,
relies on something else that will help with persistence. The `manager` is the layer which provides that, because
the `manager` already provides leadership detection and persistence for a number of other controllers such as
Ingress and Group by implementing an interceptor for the `Group` and `Controller` interfaces.

In a future PR we will make the `manager` implement the same Updatable interface, which will also persist the entire
struct into a backend of your choosing (e.g. 'swarm', 'etcd', or 'file').  This allows the simple layering of
plugins to give desired effects: some vars are durable (via wrapper/ proxy by manager) while some are in memory only
(maybe a key/ secret that is one-time and shall have no trace like a key to generate other persistable keys).

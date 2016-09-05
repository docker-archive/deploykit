hello controller
================

is a test bed

  + for Docker plugin
  + for Machete controller and driver patterns
  + for developing a reference implementation of machete controller / driver

### Building

The Makefile has a `plugin` build target.  Basic steps:
  1. Create a container
  2. Run the container on a linux host
  3. Extract the rootfs of the container (which has the binary)
  4. Set up the `manifest.json` and `plugin-config.json`.
  5. Update the state `/var/lib/docker/plugins/plugins.json`
  6. Restart docker
  7. Push plugin if the plugin is recognized.

Example

There are two plugins in this repo...  `hello` and `world`, as represented by
`hello-manifest.json` and `hello-plugins.json` and `world-manifest.json` and `world-plugins.json`.
The manifest json files could be completely parameterized (but aren't) -- so check for the image name
of the plugin (e.g. `chungers/hello-plugin`).

```
# build the hello plugin
REBUILD_VENDOR=false NAME=hello DOCKER_PUSH=true DOCKER_TAG_LATEST=true make -k plugin

# build the world plugin
REBUILD_VENDOR=false NAME=world DOCKER_PUSH=true DOCKER_TAG_LATEST=true make -k plugin
```

### Running

1. Installing the plugins

```
root@ip-172-31-6-1:~# docker plugin install chungers/hello-plugin
Plugin "chungers/hello-plugin" is requesting the following privileges:
 - network: [host]
 - mount: [/var/run]
 - mount: [/run/docker]
Do you grant the above permissions? [y/N] y
chungers/hello-plugin
```

Another one
```
root@ip-172-31-6-1:~# docker plugin install chungers/world-plugin
Plugin "chungers/world-plugin" is requesting the following privileges:
 - network: [host]
 - mount: [/var/run]
 - mount: [/run/docker]
Do you grant the above permissions? [y/N] y
chungers/world-plugin
```

Calling the plugins -- via a container
```
root@ip-172-31-6-1:~# docker run -v /var/run:/var/run -v /run/docker:/run/docker chungers/hello client chungers/hello-plugin hello.GetState
time="2016-09-05T23:01:22Z" level=info msg="Connected to engine"
time="2016-09-05T23:01:22Z" level=info msg="Looking for plugin chungers/hello-plugin"
time="2016-09-05T23:01:22Z" level=info msg="For plugin chungers/hello-plugin socket= /run/docker/cfbb742921a4f70b31f666fe32276822ff909b4ab837176decd1763bc3403420/hello.sock"
time="2016-09-05T23:01:22Z" level=info msg="Calling http://local/v1/hello.GetState via POST"
time="2016-09-05T23:01:22Z" level=info msg="Resp {\n    \"leader\": true,\n    \"name\": \"hello\",\n    \"running\": true\n  }"
{
    "leader": true,
    "name": "hello",
    "running": true
  }
```
The world plugin:
```
root@ip-172-31-6-1:~# docker run -v /var/run:/var/run -v /run/docker:/run/docker chungers/hello client chungers/world-plugin world.GetState
time="2016-09-05T23:01:58Z" level=info msg="Connected to engine"
time="2016-09-05T23:01:58Z" level=info msg="Looking for plugin chungers/world-plugin"
time="2016-09-05T23:01:58Z" level=info msg="For plugin chungers/world-plugin socket= /run/docker/6580cdb4c128c855643aad790b55685111f324121b9dba0914993e877208902e/world.sock"
time="2016-09-05T23:01:58Z" level=info msg="Calling http://local/v1/world.GetState via POST"
time="2016-09-05T23:01:58Z" level=info msg="Resp {\n    \"leader\": true,\n    \"name\": \"world\",\n    \"running\": true\n  }"
{
    "leader": true,
    "name": "world",
    "running": true
  }
```

Tell one plugin to discover the other.  This tests to see if the plugins can see each other.

Here the `hello` plugin is told to look up the `world` plugin:

```
root@ip-172-31-6-1:~# docker run -v /var/run:/var/run -v /run/docker:/run/docker chungers/hello client chungers/hello-plugin hello.Discover '{"name":"chungers/world-plugin"}'
time="2016-09-05T23:03:05Z" level=info msg="Connected to engine"
time="2016-09-05T23:03:05Z" level=info msg="Looking for plugin chungers/hello-plugin"
time="2016-09-05T23:03:05Z" level=info msg="For plugin chungers/hello-plugin socket= /run/docker/cfbb742921a4f70b31f666fe32276822ff909b4ab837176decd1763bc3403420/hello.sock"
time="2016-09-05T23:03:05Z" level=info msg="Calling http://local/v1/hello.Discover via POST"
time="2016-09-05T23:03:05Z" level=info msg="Resp {\n    \"name\": \"chungers/world-plugin\",\n    \"socket\": \"/run/docker/6580cdb4c128c855643aad790b55685111f324121b9dba0914993e877208902e/world.sock\"\n  }"
{
    "name": "chungers/world-plugin",
    "socket": "/run/docker/6580cdb4c128c855643aad790b55685111f324121b9dba0914993e877208902e/world.sock"
  }
```

Now that we know where the `world` plugin is.  Tell the `hello` to call it:
```
root@ip-172-31-6-1:~# docker run -v /var/run:/var/run -v /run/docker:/run/docker chungers/hello client chungers/hello-plugin hello.Call '{"name":"chungers/world-plugin","socket":"/run/docker/6580cdb4c128c855643aad790b55685111f324121b9dba0914993e877208902e/world.sock", "operation":"world.GetState"}'
time="2016-09-05T23:05:04Z" level=info msg="Connected to engine"
time="2016-09-05T23:05:04Z" level=info msg="Looking for plugin chungers/hello-plugin"
time="2016-09-05T23:05:04Z" level=info msg="For plugin chungers/hello-plugin socket= /run/docker/cfbb742921a4f70b31f666fe32276822ff909b4ab837176decd1763bc3403420/hello.sock"
time="2016-09-05T23:05:04Z" level=info msg="Calling http://local/v1/hello.Call via POST"
time="2016-09-05T23:05:04Z" level=info msg="Resp {\n    \"leader\": true,\n    \"name\": \"world\",\n    \"running\": true\n  }"
{
    "leader": true,
    "name": "world",
    "running": true
  }
```

In the Docker log (`/var/log/upstart/docker.log`), note the different plugin IDs.

```

INFO[0406] time="2016-09-05T23:05:04Z" level=info msg="hello - Call requested via http"   plugin=cfbb742921a4f70b31f666fe32276822ff909b4ab837176decd1763bc3403420
INFO[0406] time="2016-09-05T23:05:04Z" level=info msg="Calling http://local/v1/world.GetState via POST"   plugin=cfbb742921a4f70b31f666fe32276822ff909b4ab837176decd1763bc3403420
INFO[0406] time="2016-09-05T23:05:04Z" level=info msg="world - State requested via http"   plugin=6580cdb4c128c855643aad790b55685111f324121b9dba0914993e877208902e
```

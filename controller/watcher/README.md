Watcher
=======

Watcher is a controller that checks for changes to the SWIM config and performs actions
on controllers on the same host. The current implementation uses a URL to locate the SWIM
file.

  + Other ways of detecting changes to the SWIM file are possible.  This largely depends on
the persistence strategy used.
  + The final format of the SWIM is TBD but it will have the following properties:
    + Either through default or explicit specification, a list of controllers / drivers can be determined.
    + SWIM completely describes the configuration of each one of the controllers / drivers

On change of SWIM file, the watcher will
  + Determine a list of controllers and drivers for the running phase
  + Extract configurations for each one of the controllers from the SWIM file
  + Restart the controllers with the new configuration.


## Tentative SWIM Schema

Here's an example

```
{
    "workers" : {
        "count" : 3,
        "driver" : {
            "name" : "aws",
            "image" : "docker4x/scaler",
            "properties" : {
                "InstanceType" : "m4-xlarge"
            }
        }
    },
    "loadbalancer" : {
        "driver" : {
            "name" : "aws",
            "image" : "docker4x/controller",
            "properties" : {
                "vhosts" : {
                    "foo.com" : "TestELB",
                    "bar.com" : "InternalELB"
                }
            }
        }
    }
}
```

The watcher needs to determine a list of controllers from a SWIM.  Right now, we make it easy and make
up a section called `watcher`, that will list explicitly the containers that needs to be restarted when
SWIM configurations have changed.

In the `watcher` section, each key in the dictionary maps the container image (e.g. `docker4x/scaler`) to
another section (e.g. `scaler`) in the same file.  Watcher will then extract the section of the config
for that controller and restart it.


## Building
```
cd container
make -k container
```

## Running
```
docker run -d -v /var/run/docker.sock:/var/run/docker.sock libmachete/watcher url https://swim.com/swim.json
```

There's a test url: `https://chungers.github.io/test/config/cluster.json`

The file is served by github from the `gh-pages` branch.  Watcher will for example detect new commits and
restart the containers with new configurations.

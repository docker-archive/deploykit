Ingress Controller
==================

The Ingress Controller interfaces with the L4 plugins and Group controllers to dynamically reflect
services (routes) and backends (instances) in a stack.


## Single Load Balancer

### Synchronizing Routes

Start the simulator and all the components:

```shell
$ INFRAKIT_MANAGER_BACKEND=swarm infrakit plugin start manager simulator ingress group vanilla vars combo
```

This brings up:

```
$ infrakit


infrakit command line interface

Usage:
  infrakit [command]

Available Commands:
  combo             Access object combo which implements Flavor/0.1.0
  group             Access object group which implements Manager/0.1.0,Updatable/0.1.0,Controller/0.1.0,Group/0.1.0
  group-stateless   Access object group-stateless which implements Group/0.1.0,Metadata/0.1.0
  group/status      Access object group/status which implements Updatable/0.1.0
  group/vars        Access object group/vars which implements Updatable/0.1.0
  ingress           Access object ingress which implements Controller/0.1.0
  manager           Access the manager
  playbook          Manage playbooks
  plugin            Manage plugins
  remote            Manage remotes
  simulator/compute Access object simulator/compute which implements Instance/0.6.0
  simulator/disk    Access object simulator/disk which implements Instance/0.6.0
  simulator/lb1     Access object simulator/lb1 which implements L4/0.6.0
  simulator/lb2     Access object simulator/lb2 which implements L4/0.6.0
  simulator/lb3     Access object simulator/lb3 which implements L4/0.6.0
  simulator/net     Access object simulator/net which implements Instance/0.6.0
  template          Render an infrakit template at given url.  If url is '-', read from stdin
  up                Up everything
  util              Utilities
  vars              Access object vars which implements Updatable/0.1.0
  vanilla           Access object vanilla which implements Flavor/0.1.0
  version           Print build version information
  x                 Experimental features
```

Make sure you are in this directory so you can access the yml:

```
$ infrakit ingress controller commit -y ./example.yml
kind: ingress
metadata:
  id: ingress-singleton
  name: test.com
  tags:
    project: testing
    user: chungers
options:
  SyncInterval: 1s
properties:
- Backends:
    Groups:
    - group/workers
  L4Plugin: simulator/lb1
  RouteSources:
    swarm:
      Host: unix:///var/run/docker.sock
  Routes:
  - LoadBalancerPort: 80
    LoadBalancerProtocol: https
    Port: 8080
    Protocol: http
state: []
version: ""
```

Note that in this example we've started the ingress controller to sync with `swarm` as the
source of services (see the field `RouteSources/swarm`).  The controller syncs the routes
and backends of the L4 (`simulator/lb1` -- from the `L4Plugin` field).


```
$ infrakit simulator/lb1 backends ls
INSTANCE ID
```

```
$ infrakit simulator/lb1 routes ls
FRONTEND PORT    FRONTEND PROTOCOL      BACKEND PORT      BACKEND PROTOCOL        CERT
80               https                  8080              http
```

Create a Docker Swarm Overlay network and a service:

```
$ docker network create --driver overlay --ingress ingress
$ docker service create --network ingress --name t2 --publish 7777:80 nginx

```

Now we should see the route reflected in the simulated L4:

```
$ infrakit simulator/lb1 routes ls
FRONTEND PORT    FRONTEND PROTOCOL      BACKEND PORT      BACKEND PROTOCOL        CERT
7777             TCP                    7777              TCP
80               https                  8080              http
```

Removing the service:

```
$ docker service ls
ID                  NAME                MODE                REPLICAS            IMAGE               PORTS
kktzrhg299ek        t2                  replicated          1/1                 nginx:latest        *:7777->80/tcp
$ docker service rm t2
t2
$ docker service ls
ID                  NAME                MODE                REPLICAS            IMAGE               PORTS
```

The route should be removed:

```
$ infrakit simulator/lb1 routes ls
FRONTEND PORT    FRONTEND PROTOCOL      BACKEND PORT      BACKEND PROTOCOL        CERT
80               https                  8080              http
```

Now commit a group:

```
$ infrakit group controller commit -y ./group.yml
```

You should see:

```
$ infrakit -h


infrakit command line interface

Usage:
  infrakit [command]

Available Commands:
  combo             Access object combo which implements Flavor/0.1.0
  group             Access object group which implements Group/0.1.0,Manager/0.1.0,Updatable/0.1.0
  group-stateless   Access object group-stateless which implements Group/0.1.0,Metadata/0.1.0
  group/status      Access object group/status which implements Updatable/0.1.0
  group/vars        Access object group/vars which implements Updatable/0.1.0
  group/workers     Access object group/workers which implements Controller/0.1.0,Controller/0.1.0,Group/0.1.0
```

```
$ infrakit group/workers describe
ID                            	LOGICAL                       	TAGS
1509939754854003011           	  -                           	infrakit.config.hash=ruf47dnc2pc5p7c7ed7vzybrfmxnb3rc,infrakit.group=workers,project=infrakit,tier=web
1509939754854379446           	  -                           	infrakit.config.hash=ruf47dnc2pc5p7c7ed7vzybrfmxnb3rc,infrakit.group=workers,project=infrakit,tier=web
1509939754854683999           	  -                           	infrakit.config.hash=ruf47dnc2pc5p7c7ed7vzybrfmxnb3rc,infrakit.group=workers,project=infrakit,tier=web
1509939754853268194           	  -                           	infrakit.config.hash=ruf47dnc2pc5p7c7ed7vzybrfmxnb3rc,infrakit.group=workers,project=infrakit,tier=web
1509939754853647059           	  -                           	infrakit.config.hash=ruf47dnc2pc5p7c7ed7vzybrfmxnb3rc,infrakit.group=workers,project=infrakit,tier=web

```

And the backends:

```
$ infrakit simulator/lb1 backends ls
INSTANCE ID
1509939754854379446
1509939754854683999
1509939754853268194
1509939754853647059
1509939754854003011
```

Now scale up the group:

```
$ infrakit group/workers scale
Group workers at 5 instances
$ infrakit group/workers scale 10
Group workers at 5 instances, scale to 10
```
After some time

```
$ infrakit simulator/lb1 backends ls
INSTANCE ID
1509939754854003011
1509939894866046066
1509939894865614728
1509939894868415604
1509939754854683999
1509939754853268194
1509939754853647059
1509939894864511201
1509939894866386033
1509939754854379446
```
## Multiple Load Balancers

The schema for the ingress controller supports multiple Load Balancers.  For example, `example2.yml` has the following:

```
kind: ingress
metadata:
  name: test.com
  tags:
    project: testing
    user: chungers

# options block map to pkg/controller/ingress/types/Options
options:
  # SyncInterval is how often to sync changes between the services and the LB
  SyncInterval: 10s  # syntax is a string form of Go time.Duration

# properties block map to pkg/controller/ingress/types/Properties
properties:
  # Note that this section is missing a Vhost (so Vhost is '').  An empty Vhost entry will match all docker swarm
  # services (since we have not defined the labeling convention for indicating the vhost of a service -- so all
  # services match to ''.  This is in contrast to the Vhost of the next section, where we use a different vhost
  # so that the routes for the L4 will not pick up those from Swarm services.
  - Backends:
      Groups:
        - group/workers # This is a group at socket(group), groupID(workers).

    # This is the plugin name of the L4 plugin. When you run `infrakit plugin start ... simulator`
    # the default socket file name is 'simulator' and there's a default lb2 in the RPC object.
    L4Plugin: simulator/lb1

    # Plus all the services in Swarm that have --publish <frontend-port>:<container_port>
    RouteSources:
      swarm:
        Host: unix:///var/run/docker.sock
  - Vhost: system
    Backends:
      Groups:
        - group/workers # This is a group at socket(group), groupID(workers).

    L4Plugin: simulator/lb2

    # Here we have a static route that is always present.
    Routes:
      - LoadBalancerPort: 80
        LoadBalancerProtocol: https
        Port: 8080
        Protocol: https
	Certificate: external-cert-id  # This is an id used to identify a cert in some external system
```

Here we have two sections under `Properties`:

  + A block where the `Backends` are `group/workers`, and the `RouteSource` is from Docker Swarm; this affects the
  loadbalancer, `simulator/lb1`.
  + A second block with the same `Backends` of `group/workers`, but the routes are static; this affects the
  loadbalancer, `simulator/lb2`.

The second section also has a Vhost entry.  By default, Docker swarm services match to any blocks where the Vhost is
not specified (`""`).  So the second section, with its `Vhost` as `system`, will not pick up any routes from Docker
swarm services.

Let's try:

```
$ infrakit ingress controller commit -y ./example2.yml
kind: ingress
metadata:
  id: test.com
  name: test.com
  tags:
    project: testing
    user: chungers
options:
  SyncInterval: 10s
properties:
- Backends:
    Groups:
    - group/workers
  L4Plugin: simulator/lb1
  RouteSources:
    swarm:
      Host: unix:///var/run/docker.sock
- Backends:
    Groups:
    - group/workers
  L4Plugin: simulator/lb2
  Routes:
  - LoadBalancerPort: 80
    LoadBalancerProtocol: https
    Port: 8080
    Protocol: http
  Vhost: system
state: []
version: ""
```

Checking on the group:

```
$ infrakit group/workers scale
Group workers at 5 instances
```

Currently the services in Docker:

```
$ docker service ls
ID                  NAME                MODE                REPLICAS            IMAGE               PORTS
```

Checking on `simulator/lb1`, we expect no routes associated with this L4:

```
$ infrakit simulator/lb1 routes ls
FRONTEND PORT    FRONTEND PROTOCOL      BACKEND PORT      BACKEND PROTOCOL        CERT
$ infrakit simulator/lb1 backends ls
INSTANCE ID
1510037640975604672
1510037640974259504
1510037640974671494
1510037640975006640
1510037640975355676
```

Checking on `simulator/lb2`, we should see one route that's statically defined in the config:

```
$ infrakit simulator/lb2 routes ls
FRONTEND PORT    FRONTEND PROTOCOL      BACKEND PORT      BACKEND PROTOCOL        CERT
80               https                  8080              https                   external-cert-id
$ infrakit simulator/lb2 backends ls
INSTANCE ID
1510037640974259504
1510037640974671494
1510037640975006640
1510037640975355676
1510037640975604672
```

Now let's add a Docker Swarm service:

```
$ docker service create --network ingress --name t2 --publish 7777:80 --label lb-cert-label=lb-cert-external-id nginx
q0fknd2anvzry8dd4ovhv19n2
Since --detach=false was not specified, tasks will be created in the background.
In a future release, --detach=false will become the default.
```

Verify:

```
$ docker service ls
ID                  NAME                MODE                REPLICAS            IMAGE               PORTS
q0fknd2anvzr        t2                  replicated          1/1                 nginx:latest        *:7777->80/tcp
```

Consequently we see that the route is published to the L4, `simulator/lb1`:

```
$ infrakit simulator/lb1 routes ls
$ infrakit simulator/lb1 routes ls
FRONTEND PORT    FRONTEND PROTOCOL      BACKEND PORT      BACKEND PROTOCOL        CERT
7777             TCP                    7777              TCP                     lb-cert-external-id

$ infrakit simulator/lb1 backends ls
INSTANCE ID
1510037640975604672
1510037640974259504
1510037640974671494
1510037640975006640
1510037640975355676
```

Scaling up the group:

```
$ infrakit group/workers scale 10
Group workers at 5 instances, scale to 10
```

After some time, verify:

```
$ infrakit simulator/lb1 routes ls
FRONTEND PORT    FRONTEND PROTOCOL      BACKEND PORT      BACKEND PROTOCOL        CERT
7777             TCP                    7777              TCP                     lb-cert-external-id

$ infrakit simulator/lb1 backends ls
INSTANCE ID
1510037640975604672
1510037640975006640
1510037640975355676
1510038110991151881
1510038110991397499
1510037640974259504
1510037640974671494
1510038110991990293
1510038110991710841
1510038110990723814
```

And for `simulator/lb2`:

```
$ infrakit simulator/lb2 backends ls
INSTANCE ID
1510037640975604672
1510038110990723814
1510038110991710841
1510037640975355676
1510038110991151881
1510038110991990293
1510038110991397499
1510037640974259504
1510037640974671494
1510037640975006640

$ infrakit simulator/lb2 routes ls
FRONTEND PORT    FRONTEND PROTOCOL      BACKEND PORT      BACKEND PROTOCOL        CERT
80               https                  8080              https                   external-cert-id
```
So we see that the ingress controller can manage and synchronize the routes and backends of two different
loadbalancers.

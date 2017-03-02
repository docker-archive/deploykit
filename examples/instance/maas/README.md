# Infrakit Instance Plugin - MaaS
An example of [Infrakit](https://github.com/docker/infrakit) Instance Pluging for [Canonical MaaS](https://maas.io/) node.
Purpose of this plugin is to support Bare metal deploy with Infrakit.

## Preperation
Now this plugin need already setuped MaaS server and enlisted nodes.
And all node should be `Ready` status.


## Get start -- A demo with virtualbox

Requires:
* Needs virtualbox % sudo apt-get install virtualbox
* Needs vagrant 1.6.x
* Needs ansible installed on host machine
* MaaS Server
* API Key ( See https://maas.ubuntu.com/docs/maascli.html )
* MaaS Nodes (You need all node be `Ready` state. Register and commision manually.)
* Spec file ( example: maas-vanilla.yml )

As this is only demo, we use virtual box. we should be use BareMetal Server Cluster with MaaS

### Set up MaaS Cluster

```
$ git clone https://github.com/YujiOshima/vagrant-maas-in-a-box
$ cd vagrant-maas-in-a-box
$ vagrant up maas
```

In your browser visit http://localhost:8080/MAAS

Default login and password: `admin` `pass`

Import boot images in `Images` tab.

! Wait for import images. It takes some time. !

Set up nodes.

```
./setup-nodes.sh
```

3 nodes are enlisted in MaaS.

check `Nodes` tab.

Get MaaS API KEY. with web browser, or run `APIKEY=vagrant ssh maas -c "sudo maas-region-admin apikey --username admin"`

### Run Infrakit group, vanilla flavor plugin.

```
$ build/infrakit-group-default
```

```
$ build/infrakit-flavor-vanilla
```

Run MaaS instance plugin.

```
$ build/infrakit-maas-plugin --apikey $APIKEY --url http://localhost:8080/MAAS 
INFO[0000] Listening at: /home/ubuntu/.infrakit/plugins/instance-maas
```
Group commit!

```
$ build/infrakit group commit $GOPATH/src/github.com/YujiOshima/infrakit-maas-plugin/maas-vanilla.yml
```


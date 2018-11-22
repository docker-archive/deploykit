VMWScript Playbook Example
==========================

This is a simple playbook example to build a Kubernetes template and deploy a kubernetes cluster using the VMWScript engine in infrakit.

### How to add this playbook:

```
infrakit playbook add kube https://raw.githubusercontent.com/docker/infrakit/master/examples/playbooks/vmwscript/kubernetes/index.yml
or (from base of github clone)
infrakit playbook add kube file://`pwd`/examples/playbooks/vmwscript/kubernetes/index.yml
```

### Using the playbook

Now that the playbook has been added as `kube` you can access it:

```
infrakit use kube -h
```

The `infrakit use kube build-template` command will ask for credentials and other required configuration details along 
with the name of an *existing* VMware CentOS template. It will then create a new template that is configured to deploy
kubernetes on top of.

The `infrakit use kube deploy-cluster` will again ask for credentials and other required configuration details along 
with the prefix that will be used for all of the kubernets cluster VMs. 
e.g. {prefix}-worker01

All playbook commands support the flags `--print-only` and `--test`.  When `--print-only` is used, the playbook will
print the input to the backend without actually executing anything.  The `--test` flag is interpreted by the backends
to mean a dry run.  This may involve validation of data without actually running anything.

For more details on what you can include in the playbook templates, see the [Sprig](http://masterminds.github.io/sprig/)
documentation for template functions, as well as, the Golang [template doc](https://golang.org/pkg/text/template/).
There are also additional template functions such as include, source, and accessing infrakit services
([doc](https://github.com/docker/infrakit/blob/master/pkg/template/funcs.go#L399)).


--
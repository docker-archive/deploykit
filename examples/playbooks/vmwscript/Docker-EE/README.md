VMWScript Playbook Example
==========================

This is a simple playbook example to build Docker EE template and launch
Docker EE swarm cluster using the VMWScript engine in infrakit.

### How to add this playbook:

```
infrakit playbook add myplaybook https://raw.githubusercontent.com/docker/infrakit/master/examples/playbooks/vmwscript/Docker-EE/index.yml
or (from base of github clone)
infrakit playbook add myplaybook file://`pwd`/examples/playbooks/vmwscript/Docker-EE/index.yml
```

### Using the playbook

Now that the playbook has been added as `myplaybook` you can access it:

```
infrakit use myplaybook -h
```

The `launch-swarm` playbook depends on a VCenter template built by `build-dockerEE`.  So run `build-dockerEE` first in a
new environment.

All playbook commands support the flags `--print-only` and `--test`.  When `--print-only` is used, the playbook will
print the input to the backend without actually executing anything.  The `--test` flag is interpreted by the backends
to mean a dry run.  This may involve validation of data without actually running anything.

For more details on what you can include in the playbook templates, see the [Sprig](http://masterminds.github.io/sprig/)
documentation for template functions, as well as, the Golang [template doc](https://golang.org/pkg/text/template/).
There are also additional template functions such as include, source, and accessing infrakit services
([doc](https://github.com/docker/infrakit/blob/master/pkg/template/funcs.go#L399)).


--
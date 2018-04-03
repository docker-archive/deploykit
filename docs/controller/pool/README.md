Pool Controller
===============

Install infrakit

```
$ curl -sSL https://docker.github.io/infrakit/install | sh
Building for mac darwin / amd64
Building infrakit GOOS=darwin GOARCH=amd64, version=f9310606.m, revision=f9310606c5e1ae6afa2d7d46ffbc110351da1e67, buildtags=builtin providers
```

2. Add this playbook
```
infrakit playbook add pool https://raw.githubusercontent.com/docker/infrakit/master/docs/controller/pool/playbook.yml
```

2. Verify playbook added
```
infrakit use pool
```

3. Start up server
```
infrakit use pool start --accept-defaults
```

4. View objects in the pool controller.  In another window

```
watch -d infrakit local mystack/pool describe -o
```

5. In another shell, commit the config yml:
```
infrakit use pool workers.yml | infrakit local mystack/pool commit -y -
```

You will be prompted to answer the size of the collection and parallelism:

```
$ infrakit use pool workers.yml | infrakit local mystack/pool commit -y -
How many? [5]: 10
How many at a time? [1]: 3
```

In the window where you are watching the objects, you should see
resources being provisioned.

6. Scaling up / down
Simply commit the same config and change the size / parallelism:

```
$ infrakit use pool workers.yml | infrakit local mystack/pool commit -y -
How many? [5]: 4
How many at a time? [1]: 2
```

The undesired instances will be marked `UNMATCHED` and then eventually
terminated, using the batch size you specified.

You can verify the instances:

```
$ infrakit local az1/compute describe
```

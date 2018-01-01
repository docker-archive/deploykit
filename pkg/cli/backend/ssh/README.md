SSH Backend
===========

The SSH backend exports common flags for SSH (eg. host:port, user, password, keyfile).
These flags are taken from the command line when a playbook is invoked.  The script
itself can optionally define additional flags, as shown by the `message` flag
in the example `test.ikt`

Note that host:port is a string slice so multiple remotes are supported.  When multiple
remotes are specified, it is assumed that the authentication method applies to *all* of
the hosts.  The script will be run in parallel, and execution completes when work
on each remote host completes (either successfully or with errors).

## Authentication

Three authentication methods are supported:

  1. `keyfile` -- takes precedence over password auth, if both flags are present.
  2. `password` -- is the password to use for all remotes
  3. agent -- SSH agent is used when *neither* `keyfile` nor `password` flags are specified.

## Running

### 1. Add the Test Script as Playbook

```
infrakit playbook add ssh-test $(pwd)/pkg/cli/backend/ssh/test.ikt
```

### 2. Verify:

```
infrakit use ssh-test -h

...

Usage:
  infrakit use ssh-test [flags]

Flags:
      --hostport stringSlice   Host:port eg. localhost:22
      --keyfile string         keyfile e.g. $HOME/.ssh/id_rsa
      --message string
      --password string        password
      --print-only             True to print the rendered input
      --test                   True to do a trial run
      --user string            username

```

### 3. Test Run

```
$ infrakit use ssh-test --hostport localhost:22 --hostport server1:22 \
> --user test --keyfile $HOME/.ssh/id_rsa --message 'hello world' --test
script options
runtime cli flags
--hostport [localhost:22 server1:22]
--user test
--password
--keyfile /Users/davidchung/.ssh/id_rsa
runtime cli args
script


#!/bin/sh

echo "The message is hello world"

# Do something here
echo "I am $(whoami) running on $(hostname)"
```



### 4. Test SSH Server in Docker Container

Run a test server with password ('root'), which listens at localhost port 2222

```
docker run -d -p 2222:22 sickp/alpine-sshd:7.5
```

Verify with password `root`:

```
ssh root@localhost -p 2222
```

One with public key (at `./testkey.pub`), at localhost port 2223:
```
docker run -d -p 2223:22 -v $(pwd)/testkey.pub:/root/.ssh/authorized_keys sickp/alpine-sshd:7.5

```

Verify with private key:

```
ssh root@localhost -p 2223 -i $(pwd)/testkey
```


Now with the two containers running sshd, we can run the playbook for real.

Trying this on the sshd with password auth:

```
$ infrakit use ssh-test --hostport localhost:2222 --user root --password root --message hello
The message is hello
I am root running on 23a4b5b69e6a
```

With keyfile:

```
$ infrakit use ssh-test --hostport localhost:2223 --user root --keyfile $(pwd)/testkey --message hello
The message is hello
I am root running on 578e5ab2478d
```

### Running Against Multiple Remotes

Since container at 2223 can also authenticate via password (`root`), we can execute
the script against both 2222 and 2223:

```
$ infrakit use ssh-test --user root --password root --message MULTIPLE --hostport localhost:2222 --hostport localhost:2223 --log 5
DBUG[12-31|16:13:14] Running                                  module=cli/playbook command=ssh-test url=file:///Users/davidchung/project6/src/github.com/docker/infrakit/pkg/cli/backend/ssh/test.ikt args=[] fn=github.com/docker/infrakit/pkg/cli/playbook.list.func1
DBUG[12-31|16:13:14] running                                  module=cli/core script="\n\n\n\n\n\n\n#!/bin/sh\n\necho \"The message is MULTIPLE\"\n\n# Do something here\necho \"I am $(whoami) running on $(hostname)\"\n" fn=github.com/docker/infrakit/pkg/cli.(*Context).Execute
DBUG[12-31|16:13:14] using password auth                      module=cli/backend/ssh user=root fn=github.com/docker/infrakit/pkg/cli/backend/ssh.Script.func1
DBUG[12-31|16:13:14] running                                  module=cli/backend/ssh remote=localhost:2222 fn=github.com/docker/infrakit/pkg/cli/backend/ssh.Script.func1
DBUG[12-31|16:13:14] running                                  module=cli/backend/ssh remote=localhost:2223 fn=github.com/docker/infrakit/pkg/cli/backend/ssh.Script.func1
DBUG[12-31|16:13:14] connect                                  module=util/ssh conn="&{client:<nil> Remote:localhost:2223 Config:0xc4201ec240}" V=300 fn=github.com/docker/infrakit/pkg/util/ssh.(*Conn).connect
DBUG[12-31|16:13:14] connect                                  module=util/ssh conn="&{client:<nil> Remote:localhost:2222 Config:0xc4201ec240}" V=300 fn=github.com/docker/infrakit/pkg/util/ssh.(*Conn).connect
DBUG[12-31|16:13:14] sh                                       module=cli/backend/ssh cmd=/bin/sh fn=github.com/docker/infrakit/pkg/cli/backend/ssh.execScript
DBUG[12-31|16:13:14] exec                                     module=util/exec command=[/bin/sh] fn=github.com/docker/infrakit/pkg/util/exec.(*Builder).Prepare
The message is MULTIPLE
I am root running on 578e5ab2478d
DBUG[12-31|16:13:14] sh                                       module=cli/backend/ssh cmd=/bin/sh fn=github.com/docker/infrakit/pkg/cli/backend/ssh.execScript
DBUG[12-31|16:13:14] exec                                     module=util/exec command=[/bin/sh] fn=github.com/docker/infrakit/pkg/util/exec.(*Builder).Prepare
The message is MULTIPLE
I am root running on 23a4b5b69e6a
```

Verify that indeed the scripts ran on the two different containers -- the hostnames are the container ids.

```
$ docker ps -a | grep 22/tcp
578e5ab2478d        sickp/alpine-sshd:7.5   "/entrypoint.sh"         5 hours ago         Up 5 hours                  0.0.0.0:2223->22/tcp   reverent_poincare
23a4b5b69e6a        sickp/alpine-sshd:7.5   "/entrypoint.sh"         5 hours ago         Up 5 hours                  0.0.0.0:2222->22/tcp   upbeat_morse
$ infrakit use ssh-test --user root --password root --message MULTIPLE --hostport localhost:2222 --hostport localhost:2223
The message is MULTIPLE
I am root running on 23a4b5b69e6a
The message is MULTIPLE
I am root running on 578e5ab2478d
```
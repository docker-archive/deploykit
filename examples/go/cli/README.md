Using Infrakit Playbooks in Your Custom CLI
===========================================

The `main.go` in this directory shows how you can incorporate Infrakit
playbooks in your own custom go program.

Infrakit playbooks are template files that are rendered as input to some
supported backend.   The tempates can be bound to command
line flags and sub-commands via pre-defined template functions.  For example:

```
{{/* Render this document and then post to httpbin */}}
{{/* Add this as a playbook command via infrakit playbook add test url-to-this-file */}}

{{ $method := flag "http-method" "string" "POST" | prompt "http method?" "string" "POST" }}
{{ $message := flag "message" "string" "" | prompt "message?" "string" "" }}

{{ (cat `https://httpbin.org/` (lower $method) | nospace ) | var `url` }}
{{ var `method` $method }}

{{/* =% http (var `method`) (var `url`) `Content-Type=application/json`  %= */}}

{
  "message" : "{{ $message }}"
}

```

This is a playbook that uses the `http` backend (via the `{{/* =% http %= */}}`)
directive in the document.  Template variables are bound to command line flags
via the `flag` and `prompt` functions.  When executed the CLI will capture
user input via flag or prompt to render the template as input to the backend.
So in this case, the CLI will render the JSON document and does a HTTP request
to the backend service (`https://httpbin.org`).

You can construct multiple commands like this in `playbook.yml`:

```
# This is an index file for a playbook module that contains
# multiple commands

httpbin : httpbin.json

lines : lines.sh
```

There are two commands `httpbin` and `lines` in this playbook.

## How to build your own CLI

The `main.go` program shows the basics:

1. Load the backends you want to support via anonymous imports.  For example:

```
import (
	_ "github.com/docker/infrakit/pkg/callable/backend/http"
	_ "github.com/docker/infrakit/pkg/callable/backend/print"
	_ "github.com/docker/infrakit/pkg/callable/backend/sh"
	_ "github.com/docker/infrakit/pkg/callable/backend/ssh"
)
```

Will provide support in your program for backends like `{{/* =% http %= */}}`
or `{{/* =% ssh %= */}}`.

2. In this example, we want the first argument to be a URL for a playbook.
Use this URL to load the playbook:

```
	playbooks, err := playbook.NewModules(
		scope.Nil,
		playbook.Modules{
			playbook.Op(prog): playbook.SourceURL(makeURL(os.Args[1])),
		},
		os.Stdin, template.Options{})

```

3. The playbooks `List()` function will return a list of `cobra.Command`
for adding to your CLI.  In this case we only have one URL, so we'd have
only one command.

4. Set the args for the command returned.  In this case, we used the first
and second args (first is the program name and second is the URL itself),
so we want to set the args to `os.Args[2:]` to the returned command so that
it thinks it's executing like a program where it's `os.Args[0]`.

5. Execute the command.


## Build the program

```
go build -o mycli github.com/docker/infrakit/examples/go/cli
```

Now you should have a binary called `mycli`.

If the URL points to a single playbook command file:

```
$ ./mycli ./examples/go/cli/httpbin.json --help
mycli

Usage:
  mycli [flags]

Flags:
      --accept-defaults               True to accept defaults of prompts and flags
      --http-method string            POST
      --log int                       log level (default 4)
      --log-caller                    include caller function (default true)
      --log-debug-V int               log debug verbosity level. 0=logs all
      --log-debug-match stringSlice   debug mode only -- select records with any of the k=v pairs
      --log-debug-match-exclude       True to exclude; otherwise only include matches
      --log-format string             log format: logfmt|term|json (default "term")
      --log-stack                     include caller stack
      --log-stdout                    log to stdout
      --message string
      --print-only                    True to print the rendered input
      --test                          True to do a trial run

```
Running it:

```
$ ./mycli ./examples/go/cli/httpbin.json
http method? [POST]:
message? : hello
{
  "args": {},
  "data": "\n\n\n\n\n\n\n\n\n\n\n{\n  \"message\" : \"hello\"\n}\n",
  "files": {},
  "form": {},
  "headers": {
    "Accept-Encoding": "gzip",
    "Connection": "close",
    "Content-Length": "37",
    "Content-Type": "application/json",
    "Host": "httpbin.org",
    "User-Agent": "infrakit-cli/0.5"
  },
  "json": {
    "message": "hello"
  },
  "origin": "76.184.96.167",
  "url": "https://httpbin.org/post"
}

```

If the URL points to an index yml:
```
$ ./mycli ./examples/go/cli/playbook.yml
mycli

Usage:
  mycli [command]

Available Commands:
  httpbin     httpbin
  lines       lines
```

Running the subcommands:

```
$ ./mycli ./examples/go/cli/playbook.yml lines --header hello --lines 5
hello 1
hello 2
hello 3
hello 4
hello 5
```

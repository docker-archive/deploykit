#!/bin/sh

{{/* =% sh %= */}}

# This assumes there's a file that has the auth token and project id like
# {
#  "token": "Z37q1Ua8zLvquNUyq2a6QiWSJf5jDR9C",
#  "projectID": "6f3ec5a6-1a90-4675-b27a-3727448730f7"
# }


{{ $project := var "/project" | flag `project` `string` `` | prompt `Project?` `string` `myproject`}}
{{ $authFile := list `file://` (env `HOME`) `/.config/packet/auth.json` | join `` }}
{{ $auth := include $authFile | jsonDecode }}

{{ $t := flag `token` `string` `Token` | prompt `Token?` `string` $auth.token }}
{{ $p := flag `project-id` `string` `Project ID` | prompt `Project ID?` `string` $auth.projectID }}

infrakit-instance-packet \
    --project-id {{ $p }} --access-token {{ $t }} \
    --namespace-tags {{cat "infrakit.scope=" $project | nospace}} \
    --log 5 > {{env `INFRAKIT_HOME`}}/logs/instance-packet.log 2>&1 &

{{ var `started-packet` true }}

tail -f {{env `INFRAKIT_HOME`}}/logs/instance-packet.log

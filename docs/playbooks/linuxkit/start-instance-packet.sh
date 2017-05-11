#!/bin/bash

{{/* =% sh %= */}}

{{ $defaultImage := var `infrakit-image` | default `infrakit/devbundle` }}
{{ $packet := flag "start-packet" "bool" "Start PACKET plugin" | prompt "Start PACKET plugin?" "bool" "no" }}

{{ $authFile := list `file://` (env `HOME`) `/.config/packet/auth.json` | join `` }}
{{ $auth := include $authFile | jsonDecode }}

{{ $t := flag `token` `string` `Token` | cond $packet | prompt `Packet API Token?` `string` $auth.token }}
{{ $p := flag `project-id` `string` `Project ID` | cond $packet | prompt `Packet Project ID?` `string` $auth.projectID }}

{{ $packetImage := var `infrakit-image` | flag `packet-image` `string` `Docker image` | cond $packet | prompt `Packet plugin Docker Image?` `string` $defaultImage }}
{{ $project := var "/project" }}

{{ if $packet }}

echo "Starting PACKET plugin"

# Starting docker container for instance plugin
docker run -d --volumes-from infrakit --name instance-packet \
       {{$packetImage}} infrakit-instance-packet  --log 5 \
       --namespace-tags {{cat "infrakit.scope=" $project | nospace}} \
       --project-id {{ $p }} --access-token {{ $t }}

{{ var `started-packet` true }}

{{ end }}

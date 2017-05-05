#!/bin/bash

{{/* =% sh %= */}}

{{ $hyperkit := flag "start-hyperkit" "bool" "Start HYPERKIT plugin" | prompt "Start HYPERKIT plugin?" "bool" "no" }}

{{ $project := var "/project" }}

{{ if $hyperkit }}

echo "Starting HYPERKIT plugin.  This must be running on the Mac as a daemon and not as a container"
echo "This plugin is listening at localhost:24865"

infrakit-instance-hyperkit --log 5 > {{env `INFRAKIT_HOME`}}/logs/instance-hyperkit.log 2>&1 &

# hack to make this work with Docker4Mac -- if connecting locally on the host
echo "tcp://localhost:24865" > {{ env `INFRAKIT_HOME`}}/plugins/instance-hyperkit-local.listen

{{ end }}

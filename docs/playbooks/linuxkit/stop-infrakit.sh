# Stops all infrakit containers

{{/* This is a directive that says run this template as a sh script */}}
{{/* =% sh %= */}}

{{/* We use a prompt to ask the user if we really want to stop. Note the nil at the end is required. */}}
{{ $ok := prompt "Are you really sure you want to stop infrakit?" "bool" "no" nil }}

{{ if $ok }}

echo "Stopping Infrakit"

docker ps -f ancestor=infrakit/devbundle:dev -qa | xargs docker stop
docker ps -f ancestor=infrakit/devbundle:dev -qa | xargs docker rm

docker ps -f ancestor=infrakit/gcp:dev -qa | xargs docker stop
docker ps -f ancestor=infrakit/gcp:dev -qa | xargs docker rm

echo "Stopping local hyperkit plugin"

export INFRAKIT_HOME={{ env `INFRAKIT_HOME` }}
infrakit plugin stop --all

rm -f $INFRAKIT_HOME/plugins/instance-hyperkit-local.listen

{{ else }}

echo "Not stopping Infrakit"

{{ end }}

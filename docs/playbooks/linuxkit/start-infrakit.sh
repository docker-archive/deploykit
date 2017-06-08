# Starts all infrakit containers

{{/* =% sh %= */}}

{{ $image := flag "infrakit-image" "string" "Infrakit image" | prompt "Infrakit image?" "string" "infrakit/devbundle" }}
{{ $port := flag "infrakit-port" "int" "Infrakit mux port" | prompt "Infrakit port for remote access?" "int" 24864 }}

{{ $project := flag "project" "string" "Project name" | prompt "What's the name of the project?" "string" "testproject"}}


{{/* global variable used by other sourced templates */}}
{{ var "infrakit-image" $image }}

{{/* optional plugins */}}


echo "Starting up infrakit containers."

export INFRAKIT_HOME={{env `INFRAKIT_HOME` }}
mkdir -p $INFRAKIT_HOME/configs
mkdir -p $INFRAKIT_HOME/logs

docker run  -d --name infrakit \
       -v /var/run/docker.sock:/var/run/docker.sock \
       -v `pwd`:/project \
       -v $INFRAKIT_HOME:/infrakit \
       -p {{$port}}:24864 \
       {{ $image }} infrakit util mux --log 5

docker run  -d --volumes-from infrakit --name time {{ $image }} infrakit-event-time
docker run  -d --volumes-from infrakit --name vanilla {{ $image }} infrakit-flavor-vanilla
docker run  -d --volumes-from infrakit --name group-stateless {{ $image }} infrakit-group-default \
       --name group-stateless --poll-interval 10s

# The leader file -- only required for local store
docker run --rm --volumes-from infrakit {{ $image }} /bin/sh -c "echo group > /infrakit/leader"
docker run  -d --volumes-from infrakit --name manager \
       {{ $image }} infrakit-manager --proxy-for-group group-stateless os


{{ var "/project" $project }}

{{ source "start-instance-gcp.sh" }}

{{ source "start-instance-hyperkit.sh" }}

{{ source "start-instance-packet.sh" }}

{{ $hostsFile := list (env `INFRAKIT_HOME`) `/hosts` | join `` }}
{{ $hosts :=  include (list `file://` $hostsFile | join ``) | yamlDecode }}
{{ $_ := set $hosts `docker4mac` (list `localhost` $port | join `:`) }}
echo "Updating hosts file"
{{ $hosts | yamlEncode | file $hostsFile }}
echo "Updated hosts file"



echo "Updated hosts file.  You are using the remote `docker4mac` as defined in your hosts file in INFRAKIT_HOME/hosts"

echo "Started hyperkit: {{ var `started-hyperkit` }}"
echo "Started gcp:      {{ var `started-gcp` }}"
echo "Started packet:   {{ var `started-packet` }}"

tracked=``
# Start any tracker of resources
{{ if var `started-hyperkit`}}
tracked="$tracked --instance instance-hyperkit"
{{ end }}
{{ if var `started-gcp`}}
tracked="$tracked --instance instance-gcp"
{{ end }}
{{ if var `started-packet`}}
tracked="$tracked --instance instance-packet"
{{ end }}

if [[ "$tracked" != "" ]]; then
    echo "Tracking instances:  $tracked"
    docker run -d --volumes-from infrakit --name tracker \
	   {{ $image }} infrakit util track --name tracker $tracked
fi

# Fileserver for ipxe boot from your host
{{ $path := flag `fileserver-path` `string` `Path to serve from` | prompt "Directory to serve from?" `string` (env `PWD`) }}
{{ $port := flag `fileserver-port` `int` `Listen port` | prompt "Listening port?" `int` 8080 }}

echo "Starting up file server from {{$path}}"

docker run  -d --rm --name infrakit-fileserver \
       -v {{ $path }}:/files -p {{ $port }}:8080 \
       {{ $image }} infrakit util fileserver /files

echo "Don't forget to start up ngrok!!!"


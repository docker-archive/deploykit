# Starts all infrakit containers

{{/* =% sh %= */}}

{{ $image := flag "infrakit-image" "string" "Infrakit image" | prompt "Infrakit image?" "string" "infrakit/devbundle:dev" }}
{{ $port := flag "infrakit-port" "int" "Infrakit mux port" | prompt "Infrakit port for remote access?" "int" 24864 }}

{{ $project := flag "project" "string" "Project name" | prompt "What's the name of the project?" "string" "testproject"}}


{{/* optional plugins */}}


echo "Starting up infrakit base...  You can connect to it at infrakit -H localhost:{{$port}}"

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
       --name group-stateless --poll-interval 30s

# The leader file -- only required for local store
docker run --rm --volumes-from infrakit {{ $image }} /bin/sh -c "echo group > /infrakit/leader"
docker run  -d --volumes-from infrakit --name manager \
       {{ $image }} infrakit-manager --proxy-for-group group-stateless os


{{ var "/project" $project }}

{{ source "start-instance-hyperkit.sh" }}

{{ source "start-instance-gcp.sh" }}

echo "Updating hosts file"
{{ $hostsFile := list (env `INFRAKIT_HOME`) `/hosts` | join `` }} 
{{ $hosts :=  include (list `file://` $hostsFile | join ``) | yamlDecode }} 
{{ $_ := set $hosts `localhost` (list `localhost` $port | join `:`) }}
echo "{{ $hosts | yamlEncode }}" > {{ $hostsFile }}

{{ source "common.ikt" }}
echo # Set up infrakit.  This assumes Docker has been installed
{{ $infrakitHome := `/infrakit` }}
mkdir -p {{$infrakitHome}}/configs
mkdir -p {{$infrakitHome}}/logs
mkdir -p {{$infrakitHome}}/plugins

# dockerImage  {{ $dockerImage := var "/infrakit/docker/image" }}
# dockerMounts {{ $dockerMounts := `-v /var/run/docker.sock:/var/run/docker.sock -v /infrakit:/infrakit` }}
# dockerEnvs   {{ $dockerEnvs := `-e INFRAKIT_HOME=/infrakit -e INFRAKIT_PLUGINS_DIR=/infrakit/plugins`}}


# Cluster {{ var `/cluster/name` }} size is {{ var `/cluster/size` }} running on {{ var `/cluster/provider` }}

echo "Cluster {{ var `/cluster/name` }} size is {{ var `/cluster/size` }}"
echo "alias infrakit='docker run --rm {{$dockerMounts}} {{$dockerEnvs}} {{$dockerImage}} infrakit'" >> /root/.bashrc

alias infrakit='docker run --rm {{$dockerMounts}} {{$dockerEnvs}} {{$dockerImage}} infrakit'

echo "Starting up infrakit  ######################"
docker run -d --restart always --name infrakit -p 24864:24864 {{ $dockerMounts }} {{ $dockerEnvs }} \
       -v /var/log/:/var/log \
       -e INFRAKIT_AWS_STACKNAME={{ var `/cluster/name` }} \
       -e INFRAKIT_AWS_METADATA_POLL_INTERVAL=300s \
       -e INFRAKIT_AWS_METADATA_TEMPLATE_URL={{ var `/infrakit/metadata/configURL` }} \
       -e INFRAKIT_AWS_NAMESPACE_TAGS=infrakit.scope={{ var `/cluster/name` }} \
       -e INFRAKIT_MANAGER_BACKEND=swarm \
       -e INFRAKIT_ADVERTISE={{ var `/local/swarm/manager/logicalID` }}:24864 \
       -e INFRAKIT_TAILER_PATH=/var/log/cloud-init-output.log \
       -e INFRAKIT_GROUP_POLL_INTERVAL=30s \
       {{$dockerImage}} \
       infrakit plugin start manager group vars aws combo swarm time tailer ingress kubernetes

sleep 5

echo "Update the vars in the metadata plugin -- we put this in the vars plugin for queries later."
docker run --rm {{$dockerMounts}} {{$dockerEnvs}} {{$dockerImage}} \
       infrakit group/vars change -c \
       cluster/provider={{ var `/cluster/provider` }} \
       cluster/name={{ var `/cluster/name` }} \
       cluster/size={{ var `/cluster/size` }} \
       infrakit/config/root={{ var `/infrakit/config/root` }} \
       infrakit/docker/image={{ var `/infrakit/docker/image` }} \
       infrakit/metadata/configURL={{ var `/infrakit/metadata/configURL` }} \
       provider/image/hasDocker={{ var `/provider/image/hasDocker` }} \


{{ if eq (var `/local/swarm/manager/logicalID`) (var `/cluster/swarm/join/ip`) }}
echo "Block here to demonstrate the blocking metadata and asynchronous user update... Only on first node."

# For fun -- let's write a message for the remote CLI to see
docker run --rm {{$dockerMounts}} {{$dockerEnvs}} {{$dockerImage}} \
       infrakit group/vars change -c sys/message="To continue, please enter group/vars/spot/price using the CLI."

echo "Wait for user to remotely enter data"
docker run --rm {{$dockerMounts}} {{$dockerEnvs}} {{$dockerImage}} \
       infrakit group/vars cat spot/price --retry 5s --timeout 1.0h

docker run --rm {{$dockerMounts}} {{$dockerEnvs}} {{$dockerImage}} \
       infrakit group/vars change -c sys/message="Thank you. Continuing..."

{{ else }}
# Need time for leadership to be determined.
sleep 10
{{ end }}

echo "Rendering a view of the config groups.json for debugging."
docker run --rm {{$dockerMounts}} {{$dockerEnvs}} {{$dockerImage}} infrakit template {{var `/infrakit/config/root`}}/groups.json

#Try to commit - this is idempotent but don't error out and stop the cloud init script!
echo "Commiting to infrakit $(docker run --rm {{$dockerMounts}} {{$dockerEnvs}} {{$dockerImage}} infrakit manager commit {{var `/infrakit/config/root`}}/groups.json)"

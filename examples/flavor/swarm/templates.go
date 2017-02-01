package main

const (
	// DefaultManagerInitScriptTemplate is the default template for the init script which
	// the flavor injects into the user data of the instance to configure Docker Swarm Managers
	DefaultManagerInitScriptTemplate = `
#!/bin/sh
set -o errexit
set -o nounset
set -o xtrace

{{/* */}}
{{ include "install-docker.sh" }}

mkdir -p /etc/docker
cat << EOF > /etc/docker/daemon.json
{
  "labels": {{ INFRAKIT_LABELS | to_json }}
}
EOF

{{/* Reload the engine labels */}}
kill -s HUP $(cat /var/run/docker.pid)
sleep 5

{{ if index INSTANCE_LOGICAL_ID ALLOCATIONS.LogicalIDs | eq 0 }}

{{/* The first node of the special allocations will initialize the swarm. */}}
docker swarm init --advertise-addr {{ INSTANCE_LOGICAL_ID }}:4243

{{ else }}

  {{/* retries when trying to get the join tokens */}}
  {{ SWARM_CONNECT_RETRIES 10 "30s" }}

  {{/* The rest of the nodes will join as followers in the manager group. */}}
  docker swarm join {{ SWARM_MANAGER_IP }} --token {{  SWARM_JOIN_TOKENS.Manager }}

{{ end }}
`

	// DefaultWorkerInitScriptTemplate is the default template for the init script which
	// the flavor injects into the user data of the instance to configure Docker Swarm.
	DefaultWorkerInitScriptTemplate = `
#!/bin/sh
set -o errexit
set -o nounset
set -o xtrace

mkdir -p /etc/docker
cat << EOF > /etc/docker/daemon.json
{
  "labels": {{ INFRAKIT_LABELS | to_json }}
}
EOF

# Tell engine to reload labels
kill -s HUP $(cat /var/run/docker.pid)

sleep 10

docker swarm join {{ SWARM_MANAGER_IP }} --token {{  SWARM_JOIN_TOKENS.Worker }}
`
)

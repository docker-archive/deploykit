package main

const (
	// DefaultManagerInitScriptTemplate is the default template for the init script which
	// the flavor injects into the user data of the instance to configure Docker Swarm Managers
	DefaultManagerInitScriptTemplate = `
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

{{ if SWARM_INITIALIZED }}
docker swarm join {{ SWARM_MANAGER_IP }} --token {{  SWARM_JOIN_TOKENS.Manager }}
{{ else }}
docker swarm init
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

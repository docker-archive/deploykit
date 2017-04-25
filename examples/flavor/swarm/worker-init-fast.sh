#!/bin/sh
set -o errexit
set -o nounset
set -o xtrace

mkdir -p /etc/docker
cat << EOF > /etc/docker/daemon.json
{
  "labels": {{ INFRAKIT_LABELS | jsonEncode }}
}
EOF

# Tell engine to reload labels
kill -s HUP $(cat /var/run/docker.pid)

sleep 5

docker swarm join --token {{  SWARM_JOIN_TOKENS.Worker }} {{ SPEC.SwarmJoinIP }}:2377

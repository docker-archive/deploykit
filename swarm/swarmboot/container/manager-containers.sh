#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

MACHETE_PORT=8888

if [ ! -f config.swim ]
then
  echo 'Config.swim must exist'
  exit 1
fi

# TODO(wfarner): Move this to go code for nicer error handling at the very least.
CLUSTER_NAME=$(jq -r '.ClusterName' config.swim)
MANAGER_IPS=$(jq -c '.ManagerIPs' config.swim | tr -d '"' | tr -d '[' | tr -d ']')
NUM_WORKERS=$(jq -r '.Groups[] | select(.Type == "worker") | .Size' config.swim)
DRIVER=$(jq -r '.Driver' config.swim)
jq '.Groups[] | select(.Type == "manager") | .Config' config.swim > /scratch/manager-request.swpt

# Machete API server.
docker run \
  --detach \
  --name macheted \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --restart always \
  --publish $MACHETE_PORT:$MACHETE_PORT \
  libmachete/machete-aws \
  --cluster "$CLUSTER_NAME" --port $MACHETE_PORT $DRIVER

# Maintains manager node pool (managers 'watch' each other).
# TODO(wfarner): Join machete/quorum/scaler on a network to avoid
# the LOCAL_IP roundabout.
docker run \
  --detach \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --volume /scratch/manager-request.swpt:/manager-request.swpt:ro \
  --restart always \
  libmachete/quorum \
  run \
  $LOCAL_IP:$MACHETE_PORT $MANAGER_IPS /manager-request.swpt

# Maintains worker node pool. Use tcp listener and force discovery via flag
docker run \
  --detach --restart always \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --volume /var/run/machete/:/var/run/machete/ \
  --publish 9091:9091 \
  libmachete/scaler \
  --driver_dir /var/run/machete \
  --listen :9091 \
  run $LOCAL_IP:$MACHETE_PORT

# Watcher detects changes updates the scaler. Use tcp listener and force discovery via flag
docker run \
  --detach   --restart always \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --volume /var/run/matchete/:/var/run/machete/ \
  --publish 9090:9090 \
  libmachete/watcher \
  --driver_dir /var/run/machete \
  --discovery $LOCAL_IP:9091 \
  --listen :9090 \
  --state running \
  url $SWIM_URL

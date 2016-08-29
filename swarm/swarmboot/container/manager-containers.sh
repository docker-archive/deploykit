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

# Machete API server.
docker run \
  --detach \
  --name macheted \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --restart always \
  --publish $MACHETE_PORT:$MACHETE_PORT \
  wfarner/machete \
  --cluster "$CLUSTER_NAME" --port $MACHETE_PORT $DRIVER

# Maintains manager node pool (managers 'watch' each other).
# TODO(wfarner): Join machete/quorum/scaler on a network to avoid
# the LOCAL_IP roundabout.
docker run \
  --detach \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --volume /scratch/manager-request.swpt:/manager-request.swpt:ro \
  --restart always \
  wfarner/quorum \
  run \
  $LOCAL_IP:$MACHETE_PORT $MANAGER_IPS /manager-request.swpt

# Maintains worker node pool.
docker run \
  --detach \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --volume /scratch/worker-request.swpt:/worker-request.swpt:ro \
  --restart always \
  wfarner/scaler \
  run \
  $LOCAL_IP:$MACHETE_PORT $NUM_WORKERS /worker-request.swpt

#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

MACHETE_PORT=8888

# TODO(wfarner): Move this to go code for nicer error handling at the very least.
wget -O config.swim $SWIM_URL
CLUSTER_NAME=$(jq -r '.ClusterName' config.swim)
MANAGER_IPS=$(jq -c '.ManagerIPs' config.swim | tr -d '"' | tr -d '[' | tr -d ']')
NUM_WORKERS=$(jq -r '.NumWorkers' config.swim)
DRIVER_AND_ARGS=$(jq -r '.DriverAndArgs' config.swim)
jq '.ManagerInstance' config.swim > /scratch/manager-request.swpt
jq '.WorkerInstance' config.swim > /scratch/worker-request.swpt

# Machete API server.
docker run \
  --detach \
  --name macheted \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --restart always \
  --publish $MACHETE_PORT:$MACHETE_PORT \
  wfarner/machete \
  --cluster "$CLUSTER_NAME" --port $MACHETE_PORT $DRIVER_AND_ARGS

# Serves swarm join tokens.
docker run \
  --detach \
  --name token-server \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --restart always \
  --publish 8889:8889 \
  wfarner/tokenserver \
  run

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

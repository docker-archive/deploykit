#!/bin/bash
#
# This is a script intended to be injected into boot-time 'user data' for a Docker Swarm worker instance.

set -o errexit
set -o nounset
set -o pipefail

BOOT_LEADER=10.78.190.2

start_install() {
  sleep 5
  curl -sSL https://get.docker.com/ | sh

  # Coordinates swarm boot sequence.
  docker run \
    --detach \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    wfarner/swarmboot \
    join $BOOT_LEADER --worker
}

# See https://github.com/docker/docker/issues/23793#issuecomment-237735835 for
# details on why we background/sleep here.
start_install &

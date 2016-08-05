#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

PRIMARY=10.78.190.2

start_install() {
  sleep 5
  curl -sSL https://get.docker.com/ | sh

  docker run --detach --volume /var/run/docker.sock:/var/run/docker.sock wfarner/swarmboot join 10.78.190.2 --worker
}

# See https://github.com/docker/docker/issues/23793#issuecomment-237735835 for
# details on why we background/sleep here.
start_install &

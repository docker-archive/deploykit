#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

PRIMARY=10.78.190.2

start_install() {
  sleep 5
  curl -sSL https://get.docker.com/ | sh

  boot_args="join $PRIMARY"
  if [[ $(hostname -i) = "$PRIMARY" ]]
  then
    echo 'This is the leader, initializing cluster'
    boot_args="init"
  fi

  docker run \
    --detach \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    wfarner/swarmboot \
    $boot_args

  docker run \
    --detach \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    --publish 8889:8889 \
    wfarner/tokenserver \
    run
}

# See https://github.com/docker/docker/issues/23793#issuecomment-237735835 for
# details on why we background/sleep here.
start_install &

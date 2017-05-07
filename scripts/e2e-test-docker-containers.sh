#!/usr/bin/env bash

set -o errexit
set -o nounset

HERE="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$HERE/.."


TEST_DIR=$(pwd)/container-test
DOCKER_IMAGE="${DOCKER_IMAGE:-infrakit/devbundle}"
DOCKER_TAG="${DOCKER_TAG:-dev}"
E2E_CLEANUP=${E2E_CLEANUP:-true}

cleanup() {
  if [ "$E2E_CLEANUP" = "true" ]; then
    echo "cleaning up docker images"
    docker ps -a | grep devbundle | awk '{print $1}' | xargs docker stop
    docker ps -a | grep devbundle | awk '{print $1}' | xargs docker rm
    rm -rf $TEST_DIR
  fi
}
trap cleanup EXIT

# set up the directories
plugins=$TEST_DIR/plugins
mkdir -p $plugins
rm -rf $plugins/*

cli=$TEST_DIR/cli
mkdir -p $cli
rm -rf $cli/*

configstore=$TEST_DIR/configs
mkdir -p $configstore
rm -rf $configstore/*

mkdir -p $TEST_DIR/tutorial
rm -rf $TEST_DIR/tutorial/*


# bind mounts
volumes="-v $TEST_DIR:/ikt -v $PWD/docs:/root/docs"

# set the environment variable to use a shorter path so we don't have
# problems with Docker for Mac.  See https://github.com/docker/docker/issues/23545
envs=" -e INFRAKIT_HOME=/ikt -e INFRAKIT_PLUGINS_DIR=/ikt/plugins"

log="--log 5"

server() {
    name=$1
    shift;
    docker run -d --name $name $envs $volumes $DOCKER_IMAGE:$DOCKER_TAG $@
}

run() {
 docker run --rm $envs $volumes $DOCKER_IMAGE:$DOCKER_TAG $@
}

# set the leader -- for os / file based leader detection for manager
leaderfile=$TEST_DIR/leader
echo group > $leaderfile

# start up multiple instances of manager -- typically we want multiple SETS of plugins and managers
# but here for simplicity just start up with multiple managers and one set of plugins
server manager infrakit-manager $log --name group --proxy-for-group group-stateless os --leader-file /ikt/leader --store-dir /ikt/configs
server manager1 infrakit-manager $log --name group1 --proxy-for-group group-stateless os --leader-file /ikt/leader --store-dir /ikt/configs
server manager2 infrakit-manager $log --name group2 --proxy-for-group group-stateless os --leader-file /ikt/leader --store-dir /ikt/configs

sleep 5 # wait for leadership detection to run

server instance-file infrakit-instance-file --dir /ikt/tutorial/ $log
server group-default infrakit-group-default --poll-interval 500ms --name group-stateless $log
server flavor-vanilla infrakit-flavor-vanilla $log

sleep 2

expect_exact_output() {
  message=$1
  cmd=$2
  expected_output=$3

  actual_output="$($2)"
  echo -n "--> $message: "
  if [ "$actual_output" = "$3" ]
  then
    echo 'PASS'
  else
    echo 'FAIL'
    echo "Expected output: $expected_output"
    echo "Actual output: $actual_output"
    exit 1
  fi
}

expect_output_lines() {
  message=$1
  cmd=$2
  expected_lines=$3

  actual_line_count=$($2 | wc -l)
  echo -n "--> $message: "
  if [ "$actual_line_count" -eq "$3" ]
  then
    echo 'PASS'
  else
    echo 'FAIL'
    echo "Expected line count: $expected_lines"
    echo "Actual line count: $actual_line_count"
    exit 1
  fi
}

ls $TEST_DIR/plugins
expect_output_lines "17 plugins should be discoverable" "run infrakit plugin ls -q" "17"

expect_output_lines "0 instances should exist" "run infrakit instance describe -q --name instance-file" "0"

echo "Commiting"
run infrakit group commit /root/docs/cattle.json

echo 'Waiting for group to be provisioned'
sleep 2

expect_output_lines "5 instances should exist in group" "run infrakit group describe cattle -q" "5"
expect_output_lines "5 instances should exist" "run infrakit instance describe -q --name instance-file" "5"

echo "Free the cattle"
run infrakit group free cattle

echo "Commit again"
run infrakit group commit /root/docs/cattle.json

expect_exact_output "Should be watching one group" "run infrakit group ls -q" "cattle"

expect_exact_output \
  "Update should roll 5 and scale group to 10" \
  "run infrakit group commit /root/docs/cattle2.json --pretend" \
  "Committing cattle would involve: Performing a rolling update on 5 instances, then adding 5 instances to increase the group size to 10"

run infrakit group commit /root/docs/cattle2.json

sleep 5

expect_output_lines "10 instances should exist in group" "run infrakit group describe cattle -q" "10"

# Terminate 3 instances.
pushd $TEST_DIR/tutorial
  rm -f $(ls | head -3)
popd

sleep 5

expect_output_lines "10 instances should exist in group" "run infrakit group describe cattle -q" "10"

run infrakit group destroy cattle
expect_output_lines "0 instances should exist" "run infrakit instance describe -q --name instance-file" "0"

echo 'ALL TESTS PASSED'

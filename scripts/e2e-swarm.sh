#!/usr/bin/env bash

set -o errexit
set -o nounset

HERE="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$HERE/.."

export PATH=$PWD/build:$PATH

E2E_CLEANUP=${E2E_CLEANUP:-true}

starterpid="" # pid of the cli plugin starter
cleanup() {
  if [ "$E2E_CLEANUP" = "true" ]; then
    pgid=$(ps -o pgid= -p $starterpid)
    echo "Stopping plugin starter utility - $starterpid , pgid=$pgid"
    kill -TERM -$pgid
    echo "Stopping other jobs"
    kill $(jobs -p)
    rm -rf tutorial
  fi
}
trap cleanup EXIT

# infrakit directories
plugins=~/.infrakit/plugins
mkdir -p $plugins
rm -rf $plugins/*

configstore=~/.infrakit/configs
mkdir -p $configstore
rm -rf $configstore/*

# set the leader -- for os / file based leader detection for manager
leaderfile=~/.infrakit/leader
echo group > $leaderfile

# start up multiple instances of manager -- typically we want multiple SETS of plugins and managers
# but here for simplicity just start up with multiple managers and one set of plugins
infrakit-manager --name group --proxy-for-group group-stateless os --leader-file $leaderfile --store-dir $configstore &
infrakit-manager --name group1 --proxy-for-group group-stateless os --leader-file $leaderfile --store-dir $configstore &
infrakit-manager --name group2 --proxy-for-group group-stateless os --leader-file $leaderfile --store-dir $configstore &

sleep 5  # manager needs to detect leadership

# location of logfiles when plugins are started by the plugin cli
# the config json below expects LOG_DIR as an environment variable
LOG_DIR=~/.infrakit/logs
mkdir -p $LOG_DIR

# see the config josn 'e2e-test-plugins.json' for reference of environment variable E2E_SWARM_DIR
E2E_SWARM_DIR=~/.infrakit/e2e_swarm
mkdir -p $E2E_SWARM_DIR
rm -rf $E2E_SWARM_DIR/*

export LOG_DIR=$LOG_DIR
export E2E_SWARM_DIR=$E2E_SWARM_DIR
export SWARM_MANAGER="tcp://192.168.2.200:4243"

# note -- on exit, this won't clean up the plugins started by the cli since they will be in a separate process group
infrakit plugin start --wait --config-url file:///$PWD/scripts/e2e-test-plugins.json --os group-default instance-vagrant flavor-swarm flavor-vanilla &

starterpid=$!
echo "plugin start pid=$starterpid"

sleep 5

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

expect_output_lines "7 plugins should be discoverable" "infrakit plugin ls -q" "7"
expect_output_lines "0 instances should exist" "infrakit instance describe -q --name instance-vagrant" "0"

echo "Commiting manager"
infrakit group commit pkg/example/flavor/swarm/swarm-vagrant-manager.json

echo 'Waiting for group to be provisioned'
sleep 60

expect_exact_output "Should be watching one group" "infrakit group ls -q" "swarm-managers"
expect_output_lines "1 instances should exist in group" "infrakit group describe swarm-managers -q" "1"
expect_output_lines "1 instances should exist" "infrakit instance describe -q --name instance-vagrant" "1"

echo "Commiting workers"
infrakit group commit pkg/example/flavor/swarm/swarm-vagrant-workers.json

echo 'Waiting for group to be provisioned'
sleep 120

expect_output_lines "Should be watching two groups" "infrakit group ls -q" "2"
expect_output_lines "2 instances should exist in group" "infrakit group describe swarm-workers -q" "2"
expect_output_lines "3 instances should exist" "infrakit instance describe -q --name instance-vagrant" "3"

echo "Destroying workers"
infrakit group destroy swarm-workers
expect_output_lines "1 instances should exist" "infrakit instance describe -q --name instance-vagrant" "1"

echo "Destroying managers"
infrakit group destroy swarm-managers
expect_output_lines "0 instances should exist" "infrakit instance describe -q --name instance-vagrant" "0"

echo 'ALL TESTS PASSED'

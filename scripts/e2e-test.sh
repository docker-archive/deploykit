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

export INFRAKIT_HOME=~/.infrakit

plugins=$INFRAKIT_HOME/plugins
mkdir -p $plugins
rm -rf $plugins/*

cli=$INFRAKIT_HOME/cli
mkdir -p $cli
rm -rf $cli/*

configstore=$INFRAKIT_HOME/configs
mkdir -p $configstore
rm -rf $configstore/*

# set the leader -- for os / file based leader detection for manager
leaderfile=$INFRAKIT_HOME/leader
echo group > $leaderfile

# start up multiple instances of manager -- typically we want multiple SETS of plugins and managers
# but here for simplicity just start up with multiple managers and one set of plugins
infrakit-manager --name group --proxy-for-group group-stateless os --leader-file $leaderfile --store-dir $configstore &
infrakit-manager --name group1 --proxy-for-group group-stateless os --leader-file $leaderfile --store-dir $configstore &
infrakit-manager --name group2 --proxy-for-group group-stateless os --leader-file $leaderfile --store-dir $configstore &

sleep 5  # manager needs to detect leadership

# location of logfiles when plugins are started by the plugin cli
# the config json below expects LOG_DIR as an environment variable
LOG_DIR=$INFRAKIT_HOME/logs
mkdir -p $LOG_DIR

# see the config josn 'e2e-test-plugins.json' for reference of environment variable
INSTANCE_FILE_DIR=$INFRAKIT_HOME/instance-file
mkdir -p $INSTANCE_FILE_DIR
rm -rf $INSTANCE_FILE_DIR/*

export LOG_DIR=$LOG_DIR

# note -- on exit, this won't clean up the plugins started by the cli since they will be in a separate process group
infrakit plugin start --wait --config-url file:///$PWD/scripts/e2e-test-plugins.json --exec os group-stateless instance-file flavor-vanilla &

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

expect_output_lines "6 plugins should be discoverable" "infrakit plugin ls -q" "6"
expect_output_lines "0 instances should exist" "infrakit instance describe -q --name instance-file" "0"

echo "Commiting"
infrakit group commit docs/cattle.json

echo 'Waiting for group to be provisioned'
sleep 2

expect_output_lines "5 instances should exist in group" "infrakit group describe cattle -q" "5"
expect_output_lines "5 instances should exist" "infrakit instance describe -q --name instance-file" "5"

infrakit group free cattle
infrakit group commit docs/cattle.json

expect_exact_output "Should be watching one group" "infrakit group ls -q" "cattle"

expect_exact_output \
  "Update should roll 5 and scale group to 10" \
  "infrakit group commit docs/cattle2.json --pretend" \
  "Committing cattle would involve: Performing a rolling update on 5 instances, then adding 5 instances to increase the group size to 10"

infrakit group commit docs/cattle2.json

sleep 5

expect_output_lines "10 instances should exist in group" "infrakit group describe cattle -q" "10"

# Terminate 3 instances.
pushd $INSTANCE_FILE_DIR
  rm $(ls | head -3)
popd

sleep 5

expect_output_lines "10 instances should exist in group" "infrakit group describe cattle -q" "10"

infrakit group destroy cattle
expect_output_lines "0 instances should exist" "infrakit instance describe -q --name instance-file" "0"

echo 'ALL TESTS PASSED'

echo "Stopping plugins"
infrakit plugin stop group-default instance-file flavor-vanilla

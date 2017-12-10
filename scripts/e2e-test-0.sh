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
echo manager1 > $leaderfile

# Import env for the file backend
INFRAKIT_LEADER_FILE=$leaderfile
INFRAKIT_STORE_DIR=$configstore

# location of logfiles when plugins are started by the plugin cli
# the config json below expects LOG_DIR as an environment variable
LOG_DIR=$INFRAKIT_HOME/logs
mkdir -p $LOG_DIR

# see the config json 'e2e-test-plugins.json' for reference of environment variable
INSTANCE_FILE_DIR=$INFRAKIT_HOME/instance-file
mkdir -p $INSTANCE_FILE_DIR
rm -rf $INSTANCE_FILE_DIR/*

export LOG_DIR=$LOG_DIR

export INFRAKIT_INSTANCE_FILE_DIR=$INSTANCE_FILE_DIR
export INFRAKIT_GROUP_POLL_INTERVAL=500ms

#echo "generating logfiles"
#pkg/plugin/event/tailer/test-gen.sh $LOG_DIR/test1.log &

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
    echo "infrakit -h"
    infrakit -h
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
    echo "infrakit local -h"
    infrakit -h
    exit 1
  fi
}

note() {
    test=${TEST:-}
    echo ""
    echo ">>>>>> ${test} --- $1"
    echo ""
}

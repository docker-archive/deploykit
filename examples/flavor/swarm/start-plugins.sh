#!/usr/bin/env bash

set -o errexit
set -o nounset

HERE="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$HERE/../../.."

export PATH=$PWD/build:$PATH

INFRAKIT_HOME=${INFRAKIT_HOME:-~/.infrakit}

# infrakit directories
plugins=$INFRAKIT_HOME/plugins
mkdir -p $plugins
rm -rf $plugins/*

configstore=$INFRAKIT_HOME/configs
mkdir -p $configstore
rm -rf $configstore/*

logs=$INFRAKIT_HOME/logs
mkdir -p $logs

# set the leader -- for os / file based leader detection for manager
leaderfile=$INFRAKIT_HOME/leader
echo group > $leaderfile

export INFRAKIT_HOME=$INFRAKIT_HOME
export SWARM_MANAGER="tcp://192.168.2.200:4243"  # Note this is the IP of the leader node we provision in swarm-managers

infrakit plugin start --config-url file:///$PWD/examples/flavor/swarm/plugins.json --exec os \
	 manager \
	 group-stateless \
	 flavor-swarm \
	 instance-vagrant

sleep 5

echo "Plugins started."
echo "Do something like: infrakit manager commit file://$PWD/examples/flavor/swarm/groups-fast.json"

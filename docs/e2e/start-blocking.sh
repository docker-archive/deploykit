#!/bin/bash

HERE="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

INFRAKIT_HOME=${INFRAKIT_HOME:-$HOME/.infrakit}

LOG=$HERE/infrakit.log
rm -f $LOG

#export INFRAKIT_SIMULATOR_START_DELAY=10s
#export INFRAKIT_SIMULATOR_DESCRIBE_DELAY=60s
#export INFRAKIT_SIMULATOR_PROVISION_DELAY=10s
export INFRAKIT_MANAGER_CONTROLLERS=ingress,nfs
infrakit plugin start \
	 manager:mystack \
	 group \
	 combo \
	 vanilla \
	 simulator \
	 ingress \
	 simulator:nfs-auth \
	 enrollment:nfs \
	 vars \
	 time \
	 --log 5 --log-debug-V 500 --log-stack \
	 --log-debug-match module=manager \
	 --log-debug-match-exclude=false \

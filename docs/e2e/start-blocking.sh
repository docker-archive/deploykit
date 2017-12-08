#!/bin/bash

HERE="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

INFRAKIT_HOME=${INFRAKIT_HOME:-$HOME/.infrakit}

LOG=$HERE/infrakit.log
rm -f $LOG

export INFRAKIT_MANAGER_CONTROLLERS=ingress,enrollment
infrakit plugin start \
	 manager:mystack \
	 vars \
	 combo \
	 vanilla \
	 simulator \
	 enrollment \
	 ingress \
	 group \
	 --log 5 --log-debug-V 500 --log-stack \
	 --log-debug-match module=controller/internal \
	 --log-debug-match module=manager \
	 --log-debug-match-exclude=false \

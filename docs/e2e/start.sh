#!/bin/bash

HERE="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

INFRAKIT_HOME=${INFRAKIT_HOME:-$HOME/.infrakit}

echo "Clean up local environment"
rm -f $INFRAKIT_HOME/configs/*

LOG=$HERE/infrakit.log
rm -f $LOG

export INFRAKIT_MANAGER_CONTROLLERS=ingress,enrollment
infrakit plugin start \
	 manager:mystack \
	 group \
	 combo \
	 vanilla \
	 simulator \
	 enrollment \
	 ingress \
	 --log 5 --log-debug-V 500 --log-stack 2>$LOG &

#!/bin/bash

HERE="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

INFRAKIT_HOME=${INFRAKIT_HOME:-$HOME/.infrakit}

echo "Clean up local environment"
rm -f $INFRAKIT_HOME/configs/*

LOG=$HERE/infrakit.log
rm -f $LOG

nohup ${HERE}/start-blocking.sh &



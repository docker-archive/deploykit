#!/bin/bash

# Run this from the top level project directory
while :; do
    echo "`date` -- $RANDOM" >> ${PWD}/pkg/plugin/event/tailer/test1.log
    sleep 1
done

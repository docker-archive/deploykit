#!/bin/bash

# Run this from the top level project directory
while :; do
    date >> ${PWD}/pkg/plugin/event/tailer/test2.log
    sleep 0.5
done

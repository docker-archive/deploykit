#!/bin/bash

file=$1
while :; do
    echo "Timestamp=`date`,Value=$RANDOM" >> $file
    sleep 1
done

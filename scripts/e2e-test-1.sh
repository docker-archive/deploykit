#!/usr/bin/env bash

#note "Prepare test"
# HERE="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
# source $HERE/e2e-test-0.sh

note "Start Infrakit"

infrakit plugin start --config-url file:///$PWD/scripts/e2e-test-plugins.json \
	 manager:mystack \
	 group \
	 file:instance-file \
	 vanilla:flavor-vanilla \
	 mylogs:testlogs=inproc &

starterpid=$!

note "Plugin start pid=$starterpid"

sleep 5

found=$(infrakit local -h | grep 'testlogs')
if [ "$found" = "" ]; then
    echo "tailer not started"
    exit 1
fi


note "Starting test"

expect_output_lines "14 plugins should be discoverable" "infrakit plugin ls -q" "14"
expect_output_lines "0 instances should exist" "infrakit local instance-file describe -q " "0"

note "Commiting"
infrakit local mystack/groups commit scripts/cattle.json

note 'Waiting for group to be provisioned'
sleep 10

expect_output_lines "5 instances should exist in group" "infrakit local mystack/cattle describe -q" "5"
expect_output_lines "5 instances should exist" "infrakit local instance-file describe -q " "5"

infrakit local mystack/cattle free

note "Freed cattles; committing again"

infrakit local mystack/groups commit scripts/cattle.json

sleep 10

if [[ $(infrakit local -h | grep mystack/cattle) == "" ]]; then
    echo "checking the CLI"
    infrakit local -h
fi

expect_exact_output "Should be watching one group" "infrakit local mystack/groups ls -q" "cattle"

note "Updating specs to scale group to 10"

expect_exact_output \
  "Update should roll 5 and scale group to 10" \
  "infrakit local mystack/cattle commit scripts/cattle2.json --pretend" \
  "Committing cattle would involve: Performing a rolling update on 5 instances, then adding 5 instances to increase the group size to 10"

infrakit local mystack/cattle commit scripts/cattle2.json

sleep 10

expect_output_lines "10 instances should exist in group" "infrakit local mystack/cattle describe -q" "10"

note "Terminate 3 instances."

pushd $INSTANCE_FILE_DIR
  rm $(ls | head -3)
popd

sleep 10

expect_output_lines "10 instances should exist in group" "infrakit local mystack/cattle describe -q" "10"

infrakit local mystack/cattle destroy

sleep 10
expect_output_lines "0 instances should exist" "infrakit local instance-file describe -q " "0"

note "Stopping plugins"
infrakit plugin stop --all

sleep 2

echo ""
echo '**************************** ALL TESTS PASSED *******************************************************'

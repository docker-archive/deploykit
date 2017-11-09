#!/usr/bin/env bash

HERE="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source $HERE/e2e-test-0.sh

note "E2E Tests"

export TEST="Test 1"
. $HERE/e2e-test-1.sh



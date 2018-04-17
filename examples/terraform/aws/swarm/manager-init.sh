#!/bin/bash

set -o errexit
set -o nounset
set -o xtrace

{{/* Before we call the common boot sequence, set a few variables */}}

{{ var "/cluster/swarm/initialized" (ne 0 INDEX.Sequence) }}
{{ var "/cluster/swarm/join/ip" INSTANCE_LOGICAL_ID }}

{{ var "/local/swarm/manager/logicalID" INSTANCE_LOGICAL_ID }}

{{ var "/local/docker/engine/labels" INFRAKIT_LABELS }}
{{ var "/local/docker/swarm/join/addr" SWARM_MANAGER_ADDR }}
{{ var "/local/docker/swarm/join/token" SWARM_JOIN_TOKENS.Manager }}

{{ var "/local/infrakit/role/manager" true }}
{{ var "/local/infrakit/role/worker" false }}

{{ include "boot.sh" }}

# Append commands here to run other things that makes sense for managers

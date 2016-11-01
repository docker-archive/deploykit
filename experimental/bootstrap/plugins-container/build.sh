#!/bin/sh

set -o errexit
set -o nounset
set -o xtrace

go get github.com/docker/infrakit github.com/docker/infrakit.aws || true

cd /go/src/github.com/docker/infrakit
make binaries
cd build
mv infrakit /build/
mv infrakit-flavor-combo /build/
mv infrakit-flavor-swarm /build/
mv infrakit-flavor-vanilla /build/
mv infrakit-group-default /build/

cd /go/src/github.com/docker/infrakit.aws
make binaries
mv build/infrakit-instance-aws /build/

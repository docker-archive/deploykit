#!/bin/bash

{{/* =% sh %= */}}

docker run --rm -e GOARCH=amd64 -e GOOS=darwin -v `pwd`:/build infrakit/installer build-hyperkit
cp ./infrakit-instance-hyperkit /usr/local/bin

echo "Installed hyperkit plugin"
infrakit-instance-hyperkit -h

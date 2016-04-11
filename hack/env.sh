#!/bin/bash

: ${PROJECT:=`basename $PWD`}
echo "Current directory is $(pwd). Project=${PROJECT}"

# Set the GOPATH
# The convention is that somewhere in the path there's a go/src/ directory.  We will set
# the root of that as the GOPATH
export GOPATH="$(pwd | awk -F '/go/src/' '{print $1}')/go"
export PATH=$GOPATH/bin:$PATH

# Godep dependency manager
if [[ $(which godep) == "" ]]; then
    echo "Installing godep."
    go install github.com/tools/godep
else
    echo "Found godep"
fi

if [[ $(which oracle) == "" ]]; then
    echo "Setting up go oracle for source code analysis."
    go install golang.org/x/tools/oracle
else
    echo "Found go oracle"
fi

echo "GOPATH=${GOPATH}"
echo "PATH=${PATH}"
echo "GO Binary: $(which go)"
go version


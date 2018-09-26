#!/bin/bash

# This script adds the canonical import annotation to each .go file
# For example,
#
# import foo
# in the beginning of the source file is replaced with
#
# import foo // import "githubcom/my/repo/pkg/foo"
# where the package path is the original package name.
#
# This is used to add canonical import paths to all the source files prior to moving a repo
# to a new github org, without breaking the build (since without the canonical imports, the
# GOPATH will have to change and won't match the package references.

PACKAGES=$(go list ./...)

for i in $PACKAGES; do
    originalImport=$i
    canonical=$(echo $originalImport | sed -e 's/docker\/infrakit/infrakit\/infrakit/g')
    echo "Processing package ${dockerImport} ==> ${canonical}"
    src=$(ls $GOPATH/src/$i/*.go)
    for s in $src; do
	echo "Processing file $s"
	pkg=$(basename $(dirname $s))
	if [ $pkg != "main" ]; then
	    search="package $pkg"
	    replace="package $pkg \/\/ import \"$(echo $originalImport | sed -e 's/\//\\\//g')\""
	    sedexpr="s/$search/$replace/g"
	    echo sed -e "'$sedexpr'" $s | sh > /tmp/buff
	    mv /tmp/buff $s
	fi
    done
done

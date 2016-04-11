#!/bin/bash

# Git commit hash / message
export GIT_REPO=$(git config --get remote.origin.url | sed -e 's/[\/&]/\\&/g')
export GIT_TAG=$(git describe --abbrev=0 --tags)
export GIT_COMMIT_HASH=$(git rev-list --max-count=1 --reverse HEAD)
export GIT_COMMIT_MESSAGE=$(git log -1 | tail -1 | sed -e "s/^[ ]*//g")
export BUILD_TIMESTAMP=$(date +"%Y-%m-%d-%H:%M")


if [[ "$@" == "" ]]; then
    echo "No file to process."
    exit
fi

echo "Git commit $GIT_REPO @ $GIT_COMMIT_HASH ($GIT_COMMIT_MESSAGE) on $BUILD_TIMESTAMP"
sed -ri "s/@@GIT_REPO@@/${GIT_REPO}/g" $@
sed -ri "s/@@GIT_COMMIT_HASH@@/${GIT_COMMIT_HASH}/g" $@
sed -ri "s/@@GIT_COMMIT_MESSAGE@@/${GIT_COMMIT_MESSAGE}/g" $@
sed -ri "s/@@BUILD_TIMESTAMP@@/${BUILD_TIMESTAMP}/g" $@
sed -ri "s/@@BUILD_NUMBER@@/${CIRCLE_BUILD_NUM}/g" $@

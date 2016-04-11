##############################################################################
#
# Targets for generate build info
#

GIT_REPO=`git config --get remote.origin.url | sed -e 's/[\/&]/\\&/g'`
GIT_TAG=`git describe --abbrev=0 --tags`
GIT_BRANCH=`git rev-parse --abbrev-ref HEAD`
GIT_COMMIT_HASH=`git rev-list --max-count=1 --reverse HEAD`
GIT_COMMIT_MESSAGE=`git log -1 --format="%h,%an,%s"`
BUILD_TIMESTAMP=`date +"%Y-%m-%d-%H:%M"`
BUILD_NUMBER?=0
BUILD_LABEL?=`basename $$(git config --get remote.origin.url) | sed -e s/.git//g`

LDFLAGS=\
-X github.com/conductant/gohm/pkg/version.gitRepo=$(GIT_REPO) \
-X github.com/conductant/gohm/pkg/version.gitTag=$(GIT_TAG) \
-X github.com/conductant/gohm/pkg/version.gitBranch=$(GIT_BRANCH) \
-X github.com/conductant/gohm/pkg/version.gitCommitHash=$(GIT_COMMIT_HASH) \
-X github.com/conductant/gohm/pkg/version.buildTimestamp=$(BUILD_TIMESTAMP) \
-X github.com/conductant/gohm/pkg/version.buildNumber=$(BUILD_NUMBER) \
-X github.com/conductant/gohm/pkg/version.buildLabel=$(BUILD_LABEL) \

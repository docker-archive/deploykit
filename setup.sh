# Make sure that you have Go installed
# https://golang.org/dl/

# Set your development environment via environment variable, or use the default
# E.g., `export DEVENV=/Users/yourname/Code/learn_infrakit`
devenv=${DEVENV:-"/go"}

# Set up the local go compiler
export LOCALGO=`which go`
export PATH=$LOCALGO/bin:$PATH

# Set up Go PATH
mkdir -p ${devenv}
export GOPATH=${devenv}
export PATH=$GOPATH/bin:$PATH

# Create dev environment
# TODO: Do no clone or create directories if already existing
mkdir -p $devenv/src/github.com/docker
cd $devenv/src/github.com/docker
git clone git@github.com:docker/infrakit.git
cd infrakit

# Install supporting tools
go get -u github.com/kardianos/govendor  # the dependency manager
go get -u github.com/golang/lint/golint  # if you're running tests



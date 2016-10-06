# Make sure that you have Go installed
# https://golang.org/dl/

# Substitute your development folder her
DEVENV='/Users/jesse/Code/dkr_infrakit'

# Check you go compiler
export LOCALGO=`which go`

# Set up Go PATH
mkdir -p $DEVENV
export PATH=$LOCALGO/bin:$PATH
export GOPATH=!$
export PATH=$GOPATH/bin:$PATH

# Create dev environment
# TODO: Do no clone or create directories if already existing
mkdir -p $DEVENV/src/github.com/docker
cd !$
git clone git@github.com:docker/infrakit.git
cd infrakit

# Install supporting tools
go get -u github.com/kardianos/govendor  # the dependency manager
go get -u github.com/golang/lint/golint  # if you're running tests


# Set an output prefix, which is the local directory if not specified
PREFIX?=$(shell pwd -L)

# Used to populate version variable in main package.
VERSION=$(shell git describe --match 'v[0-9]*' --dirty='.m' --always)

# The Commit hash of engine-api to build against.
DOCKER_ENGINE_API_SHA1=98348ad6f9c89bb10f31ac32cd1b12cbadd292b6

# Docker image to build
DOCKER_REPO?="docker4x/controller"
DOCKER_TAG?="dev"
DOCKER_IMAGE=${DOCKER_REPO}:${DOCKER_TAG}

# Engine API dependencies
DOCKER_SRC_ROOT=${GOPATH}/src/github.com/docker

SRC_ROOT?=${DOCKER_SRC_ROOT}/libmachete

.PHONY: clean all
.DEFAULT: all
all: docker

test:
	@echo "+ $@"
	godep go test -race -v ./...

clean:
	@echo "+ $@"
	-rm -rf build
	-docker rmi -f ${DOCKER_IMAGE}  # remove local version
	-docker rmi -f ${DOCKER_REPO}:latest  # remove local versino

docker: clean dep-engine-api
	@echo "+ $@"
	-docker rmi -f build_binary
	@echo "Run from top level project directory and copy tree to build in container"
	cd ${SRC_ROOT} && docker build -t build_binary -f controller/loadbalancer/Dockerfile.build .
	docker run `docker images -q | head -1`
	docker cp `docker ps -a -q | head -1`:/build .  # Copy the built binaries back to local context
	docker build -t ${DOCKER_IMAGE} -t ${DOCKER_REPO}:latest -f ./Dockerfile .
	docker push ${DOCKER_IMAGE}
ifeq (${DOCKER_TAG_LATEST},true)
	docker push ${DOCKER_REPO}:latest
endif

dep-engine-api:
	-rm -rf ${DOCKER_SRC_ROOT}/engine-api
	@cd ${DOCKER_SRC_ROOT} && git clone git@github.com:docker/engine-api.git
	@echo "Checked out engine-api"
	@cd ${DOCKER_SRC_ROOT}/engine-api && git checkout ${DOCKER_ENGINE_API_SHA1}
	@cd ${SRC_ROOT} && godep update github.com/docker/engine-api/...
	@echo "Updated vendoring dependencies"
	@git status

# Makefile for building the loadbalancer controller
#
# We use a container to compile the binary, and in
# the container we use govendor to fetch all the versioned
# packages and dependencies.
# If you need to change the version of a package, update
# the vendor/vendor.json and the build container will
# automatically pull in the versioned dependencies without
# changing your local workspace.

# Docker image to build
DOCKER_REPO?="docker4x/controller"
DOCKER_TAG?="dev"
DOCKER_IMAGE=${DOCKER_REPO}:${DOCKER_TAG}

# Root of the project
SRC_ROOT?=$(shell pwd -L)/../..

.PHONY: clean all
.DEFAULT: all
all: docker


test:
	@echo "+ $@"
	godep go test -race -v ./...

clean:
	@echo "+ $@"
	-rm -rf build
	-@docker rmi -f ${DOCKER_IMAGE}  # remove local version
	-@docker rmi -f ${DOCKER_REPO}:latest  # remove local version

docker: clean
	@echo "+ $@"
	-docker rmi -f build_binary
	@echo "Building binary in container"
	cd ${SRC_ROOT} && docker build -t build_binary -f controller/loadbalancer/Dockerfile.build .
	@docker run `docker images -q | head -1`
	@docker cp `docker ps -a -q | head -1`:/build .  # Retrieve the build artifacts
	@echo "Package the build artifacts in container image"
	@docker build -t ${DOCKER_IMAGE} -t ${DOCKER_REPO}:latest -f ./Dockerfile .
	@docker push ${DOCKER_IMAGE}
ifeq (${DOCKER_TAG_LATEST},true)
	@docker push ${DOCKER_REPO}:latest
endif


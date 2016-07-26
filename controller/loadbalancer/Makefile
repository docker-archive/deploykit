# The Commit hash or SHA of the docker/docker repo
DOCKER_DOCKER_CHECKPOINT=v1.12.0-rc5
# The Commit hash of engine-api to build against.
DOCKER_ENGINE_API_CHECKPOINT=a52656d77f09d394104c1639824eada038bfdb89

# Docker image to build
DOCKER_REPO?="docker4x/controller"
DOCKER_TAG?="aws-v1.12.0-rc5-beta3"
DOCKER_IMAGE=${DOCKER_REPO}:${DOCKER_TAG}

# Engine dependencies via git checkout
# Note we actually update the repos in this Go workspace.

# Set the environment variable UPDATE_DEP=true make -k update-dep
# to actually change the docker engine repos in your workspace.
# Otherwise, checkout the dependencies in a local directory and copy
# the files into a container to build in the sandbox.
ifeq (${UPDATE_DEP},true)
	DOCKER_SRC_ROOT=${GOPATH}/src/github.com/docker
else
	DOCKER_SRC_ROOT=$(shell pwd -L)/deps/go/src/github.com/docker
endif

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
	-rm -rf ${DOCKER_SRC_ROOT}/docker
	-rm -rf ${DOCKER_SRC_ROOT}/engine-api
	-rm -rf build
	-rm -rf deps
	-mkdir -p ${DOCKER_SRC_ROOT}
	-@docker rmi -f ${DOCKER_IMAGE}  # remove local version
	-@docker rmi -f ${DOCKER_REPO}:latest  # remove local version

docker: clean checkout-docker-docker checkout-engine-api #update-dep
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

checkout-docker-docker:
	-rm -rf ${DOCKER_SRC_ROOT}/docker
	@cd ${DOCKER_SRC_ROOT} && git clone git@github.com:docker/docker.git
	@cd ${DOCKER_SRC_ROOT}/docker && git checkout ${DOCKER_DOCKER_CHECKPOINT}
	@echo "Checked out docker/docker"

checkout-engine-api:
	-rm -rf ${DOCKER_SRC_ROOT}/engine-api
	@cd ${DOCKER_SRC_ROOT} && git clone git@github.com:docker/engine-api.git
	@cd ${DOCKER_SRC_ROOT}/engine-api && git checkout ${DOCKER_ENGINE_API_CHECKPOINT}
	@echo "Checked out engine-api"

update-dep: checkout-docker-docker checkout-engine-api
	@cd ${SRC_ROOT} && godep update github.com/docker/docker/...
	@cd ${SRC_ROOT} && godep update github.com/docker/engine-api/...
	@echo "Updated vendoring dependencies"
	@git status

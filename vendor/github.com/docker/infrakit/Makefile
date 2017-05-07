# Set an output prefix, which is the local directory if not specified
PREFIX?=$(shell pwd -L)

# Used to populate version variable in main package.
VERSION?=$(shell git describe --match 'v[0-9]*' --dirty='.m' --always)
REVISION?=$(shell git rev-list -1 HEAD)

# Docker client API version.  Change this to be consistent with the version of the vendored sources you use.
DOCKER_CLIENT_VERSION?=1.24

# Allow turning off function inlining and variable registerization
ifeq (${DISABLE_OPTIMIZATION},true)
	GO_GCFLAGS=-gcflags "-N -l"
	VERSION:="$(VERSION)-noopt"
endif

.PHONY: clean all fmt vet lint build test vendor-update containers check-docs e2e-test get-tools
.DEFAULT: all
all: clean fmt vet lint build test binaries

ci: fmt vet lint check-docs coverage e2e-test

AUTHORS: .mailmap .git/HEAD
	git log --format='%aN <%aE>' | sort -fu > $@

# Package list
PKGS_AND_MOCKS := $(shell go list ./... | grep -v /vendor)
PKGS := $(shell echo $(PKGS_AND_MOCKS) | tr ' ' '\n' | grep -v /mock$)

# Current working environment.  Set these explicitly if you want to cross-compile
# in the build container (see the build-in-container target):
GOOS?=$(shell go env GOOS)
GOARCH?=$(shell go env GOARCH)
DOCKER_BUILD_FLAGS?=--no-cache --pull
build-in-container:
	@echo "+ $@"
	@docker build ${DOCKER_BUILD_FLAGS} -t infrakit-build -f ${CURDIR}/dockerfiles/Dockerfile.build .
	@docker run --rm \
		-e GOOS=${GOOS} -e GOARCCH=${GOARCH} -e DOCKER_CLIENT_VERSION=${DOCKER_CLIENT_VERSION} \
		-v ${CURDIR}/build:/go/src/github.com/docker/infrakit/build \
		infrakit-build

# For packaging as Docker container images.  Set the environment variables DOCKER_PUSH, DOCKER_TAG_LATEST
# if also push to remote repo.  You must have access to the remote repo.
DOCKER_IMAGE?=infrakit/devbundle
DOCKER_TAG?=dev
build-docker:
	@echo "+ $@"
	GOOS=linux GOARCH=amd64 make build-in-container
	@docker build ${DOCKER_BUILD_FLAGS} \
	-t ${DOCKER_IMAGE}:${DOCKER_TAG} \
	-f ${CURDIR}/dockerfiles/Dockerfile.bundle .
	@echo "Running tests -- scripts/e2e-test-docker-containers.sh to verify the binaries"
	@scripts/e2e-test-docker-containers.sh
ifeq (${DOCKER_PUSH},true)
	@docker push ${DOCKER_IMAGE}:${DOCKER_TAG}
ifeq (${DOCKER_TAG_LATEST},true)
	@docker tag ${DOCKER_IMAGE}:${DOCKER_TAG} ${DOCKER_IMAGE}:latest
	@docker push ${DOCKER_IMAGE}:latest
endif
endif

# Trivial installer that packages source code (via go get) and has script for building the CLI
INSTALLER_IMAGE?=infrakit/installer
INSTALLER_TAG?=$(REVISION)
build-installer:
	@echo "+ $@"
	@docker build -t ${INSTALLER_IMAGE}:${INSTALLER_TAG} -t ${INSTALLER_IMAGE}:latest \
	-f ${CURDIR}/dockerfiles/Dockerfile.installer .
ifeq (${DOCKER_PUSH},true)
	@docker push ${INSTALLER_IMAGE}:${INSTALLER_TAG}
ifeq (${DOCKER_TAG_LATEST},true)
	@docker tag ${INSTALLER_IMAGE}:${INSTALLER_TAG} ${INSTALLER_IMAGE}:latest
	@docker push ${INSTALLER_IMAGE}:latest
endif
endif

build-docker-dev:
	@echo "+ $@"
	GOOS=linux GOARCH=amd64 make build-in-container
	@docker build ${DOCKER_BUILD_FLAGS} \
	-t ${DOCKER_IMAGE}:${DOCKER_TAG} \
	-f ${CURDIR}/dockerfiles/Dockerfile.bundle .

get-tools:
	@echo "+ $@"
	@go get -u \
		github.com/golang/lint/golint \
		github.com/wfarner/blockcheck \
		github.com/rancher/trash

vet:
	@echo "+ $@"
	@go vet $(PKGS)

fmt:
	@echo "+ $@"
	@test -z "$$(gofmt -s -l . 2>&1 | grep -v ^vendor/ | tee /dev/stderr)" || \
		(echo >&2 "+ please format Go code with 'gofmt -s', or use 'make fmt-save'" && false)

fmt-save:
	@echo "+ $@"
	@gofmt -s -l . 2>&1 | grep -v ^vendor/ | xargs gofmt -s -l -w

lint:
	@echo "+ $@"
	$(if $(shell which golint || echo ''), , \
		$(error Please install golint: `make get-tools`))
	@test -z "$$(golint ./... 2>&1 | grep -v ^vendor/ | grep -v mock/ | tee /dev/stderr)"

check-docs:
	@echo "+ $@"
	find . -name '*.md' | grep -v vendor/ | blockcheck
	./scripts/doc-source-check

build:
	@echo "+ $@"
	@go build ${GO_LDFLAGS} $(PKGS)

clean:
	@echo "+ $@"
	rm -rf build
	mkdir -p build

define binary_target_template
build/$(1):
	go build -o build/$(1) \
		-ldflags "-X github.com/docker/infrakit/pkg/cli.Version=$(VERSION) -X github.com/docker/infrakit/pkg/cli.Revision=$(REVISION) -X github.com/docker/infrakit/pkg/util/docker.ClientVersion=$(DOCKER_CLIENT_VERSION)" $(2)
endef
define define_binary_target
	$(eval $(call binary_target_template,$(1),$(2)))
endef

$(call define_binary_target,infrakit,github.com/docker/infrakit/cmd/infrakit)
$(call define_binary_target,infrakit-manager,github.com/docker/infrakit/cmd/manager)
$(call define_binary_target,infrakit-group-default,github.com/docker/infrakit/cmd/group)
$(call define_binary_target,infrakit-resource,github.com/docker/infrakit/cmd/resource)
$(call define_binary_target,infrakit-flavor-combo,github.com/docker/infrakit/examples/flavor/combo)
$(call define_binary_target,infrakit-flavor-swarm,github.com/docker/infrakit/examples/flavor/swarm)
$(call define_binary_target,infrakit-flavor-vanilla,github.com/docker/infrakit/examples/flavor/vanilla)
$(call define_binary_target,infrakit-flavor-zookeeper,github.com/docker/infrakit/examples/flavor/zookeeper)
$(call define_binary_target,infrakit-instance-libvirt,github.com/docker/infrakit/cmd/instance/libvirt)
$(call define_binary_target,infrakit-instance-file,github.com/docker/infrakit/examples/instance/file)
$(call define_binary_target,infrakit-instance-terraform,github.com/docker/infrakit/examples/instance/terraform)
$(call define_binary_target,infrakit-instance-vagrant,github.com/docker/infrakit/examples/instance/vagrant)
$(call define_binary_target,infrakit-instance-maas,github.com/docker/infrakit/examples/instance/maas)
$(call define_binary_target,infrakit-instance-docker,github.com/docker/infrakit/examples/instance/docker)
$(call define_binary_target,infrakit-instance-hyperkit,github.com/docker/infrakit/cmd/instance/hyperkit)
$(call define_binary_target,infrakit-event-time,github.com/docker/infrakit/examples/event/time)

binaries: clean build-binaries
build-binaries:	build/infrakit \
		build/infrakit-manager \
		build/infrakit-group-default \
		build/infrakit-resource \
		build/infrakit-flavor-combo \
		build/infrakit-flavor-swarm \
		build/infrakit-flavor-vanilla \
		build/infrakit-flavor-zookeeper \
		build/infrakit-instance-libvirt \
		build/infrakit-instance-file \
		build/infrakit-instance-terraform \
		build/infrakit-instance-vagrant \
		build/infrakit-instance-maas \
		build/infrakit-instance-docker \
		build/infrakit-instance-hyperkit \
		build/infrakit-event-time
	@echo "+ $@"
ifneq (,$(findstring .m,$(VERSION)))
	@echo "\nWARNING - repository contains uncommitted changes, tagged binaries as dirty\n"
endif

cli: build-cli
build-cli: build/infrakit
	@echo "+ $@"
ifneq (,$(findstring .m,$(VERSION)))
	@echo "\nWARNING - repository contains uncommitted changes, tagged binaries as dirty\n"
endif

install:
	@echo "+ $@"
	@go install ${GO_LDFLAGS} $(PKGS)

generate:
	@echo "+ $@"
	@go generate -x $(PKGS_AND_MOCKS)

test:
	@echo "+ $@"
	@go test -test.short -timeout 30s -race -v $(PKGS)

coverage:
	@echo "+ $@"
	@for pkg in $(PKGS); do \
		go test -test.short -race -coverprofile="../../../$$pkg/coverage.txt" $${pkg} || exit 1; \
	done

e2e-test: binaries
	@echo "+ $@"
	./scripts/e2e-test.sh

test-full:
	@echo "+ $@"
	@go test -race $(PKGS)

vendor-update:
	@echo "+ $@"
	@trash -u

terraform-linux:
	@echo "+ $@"
	wget -O tf.zip https://releases.hashicorp.com/terraform/0.9.3/terraform_0.9.3_linux_amd64.zip && unzip tf.zip && mv terraform ./build

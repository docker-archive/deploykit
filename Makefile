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

define build_binary
	go build -o build/$(1) \
		-ldflags "-X github.com/docker/infrakit/pkg/cli.Version=$(VERSION) -X github.com/docker/infrakit/pkg/cli.Revision=$(REVISION) -X github.com/docker/infrakit/pkg/util/docker.ClientVersion=$(DOCKER_CLIENT_VERSION)" $(2)
endef

binaries: clean build-binaries
build-binaries:
	@echo "+ $@"
ifneq (,$(findstring .m,$(VERSION)))
	@echo "\nWARNING - repository contains uncommitted changes, tagging binaries as dirty\n"
endif

	$(call build_binary,infrakit,github.com/docker/infrakit/cmd/cli)
	$(call build_binary,infrakit-manager,github.com/docker/infrakit/cmd/manager)
	$(call build_binary,infrakit-group-default,github.com/docker/infrakit/cmd/group)
	$(call build_binary,infrakit-flavor-combo,github.com/docker/infrakit/examples/flavor/combo)
	$(call build_binary,infrakit-flavor-swarm,github.com/docker/infrakit/examples/flavor/swarm)
	$(call build_binary,infrakit-flavor-vanilla,github.com/docker/infrakit/examples/flavor/vanilla)
	$(call build_binary,infrakit-flavor-zookeeper,github.com/docker/infrakit/examples/flavor/zookeeper)
	$(call build_binary,infrakit-instance-file,github.com/docker/infrakit/examples/instance/file)
	$(call build_binary,infrakit-instance-terraform,github.com/docker/infrakit/examples/instance/terraform)
	$(call build_binary,infrakit-instance-vagrant,github.com/docker/infrakit/examples/instance/vagrant)

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

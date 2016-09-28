# REPO
REPO?=github.com/docker/infrakit.aws

# Set an output prefix, which is the local directory if not specified
PREFIX?=$(shell pwd -L)

# Used to populate version variable in main package.
VERSION=$(shell git describe --match 'v[0-9]*' --dirty='.m' --always)

# Allow turning off function inlining and variable registerization
ifeq (${DISABLE_OPTIMIZATION},true)
	GO_GCFLAGS=-gcflags "-N -l"
	VERSION:="$(VERSION)-noopt"
endif

.PHONY: clean all fmt vet lint build test vendor-sync containers
.DEFAULT: all
all: fmt vet lint build test

ci: fmt vet lint vendor-sync vendor-check coverage

AUTHORS: .mailmap .git/HEAD
	 git log --format='%aN <%aE>' | sort -fu > $@

# Package list
PKGS_AND_MOCKS := $(shell go list ./... | grep -v ^${REPO}/vendor/)
PKGS := $(shell echo $(PKGS_AND_MOCKS) | tr ' ' '\n' | grep -v /mock$)

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
		$(error Please install golint: `go get -u github.com/golang/lint/golint`))
	@test -z "$$(golint ./... 2>&1 | grep -v ^vendor/ | grep -v mock/ | tee /dev/stderr)"

build: vendor-sync
	@echo "+ $@"
	@go build ${GO_LDFLAGS} $(PKGS)

install: vendor-sync
	@echo "+ $@"
	@go install ${GO_LDFLAGS} $(PKGS)

generate:
	@echo "+ $@"
	@go generate -x $(PKGS_AND_MOCKS)

test: vendor-sync
	@echo "+ $@"
	@go test -test.short -race -v $(PKGS)

coverage: vendor-sync
	@echo "+ $@"
	@for pkg in $(PKGS); do \
	  go test -test.short -coverprofile="../../../$$pkg/coverage.txt" $${pkg} || exit 1; \
	done

test-full: vendor-sync
	@echo "+ $@"
	@go test -race $(PKGS)

# govendor helpers
check-govendor:
	$(if $(shell which govendor || echo ''), , \
		$(error Please install govendor: go get github.com/kardianos/govendor))

vendor-sync: check-govendor
	@echo "+ $@"
	@govendor sync

vendor-save: check-govendor
	@echo "+ $@"
	@govendor add +external

vendor-check:
	@echo "+ $@"
	@test -z "$$(govendor status | tee /dev/stderr)"

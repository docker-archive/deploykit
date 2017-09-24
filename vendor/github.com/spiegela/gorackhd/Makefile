# REPO
REPO?=github.com/spiegela/gorackhd

# Set an output prefix, which is the local directory if not specified
PREFIX?=$(shell pwd -L)

# Used to populate version variable in main package.
VERSION?=$(shell git describe --match 'v[0-9]*' --dirty='.m' --always)
REVISION?=$(shell git rev-list -1 HEAD)

GIT := /usr/local/bin/git
SWAGGER_PKG := github.com/go-swagger/go-swagger
SWAGGER_CMD := swagger
SPEC_DIR := on-http/static
SPEC_FILE := monorail-2.0.yaml
CLIENT := client
MODELS := models
MOCK := mock
CLIENT_BINDINGS := $(CLIENT)/monorail_client.go

# Allow turning off function inlining and variable registerization
ifeq (${DISABLE_OPTIMIZATION},true)
	GO_GCFLAGS=-gcflags "-N -l"
	VERSION:="$(VERSION)-noopt"
endif

.PHONY: clean all swagger_deps fmt vet lint build test get-tools \
	clean clobber
.DEFAULT: all

all: clean fmt vet lint build test

clean:
	rm -fr $(COVER_OUT)

clean_generated:
	rm -fr $(CLIENT) $(MODELS) $(MOCK)

clobber: clean
	rm -fr vendor

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

lint:
	@echo "+ $@"
	$(if $(shell which golint || echo ''), , \
		$(error Please install golint: `go get -u github.com/golang/lint/golint`))
	@test -z "$$(golint ./... 2>&1 | grep -v ^vendor/ | grep -v ^mock/ | grep -v ^client/ | grep -v ^models/ | tee /dev/stderr)"

fmt-save:
	@echo "+ $@"
	@gofmt -s -l . 2>&1 | grep -v ^vendor/ | xargs gofmt -s -l -w

build:
	@echo "+ $@"
	@go build ${GO_LDFLAGS} $(PKGS)

generate:
	$(if $(shell which mockgen || echo ''), , \
		$(error Please install golint: `go get -u github.com/golang/mock/mockgen`))
	$(if $(shell which $(SWAGGER_CMD) || echo ''), , \
		$(error Please install golint: `go get -u $(SWAGGER_PKG)`))
	@cd $(SPEC_DIR) && $(SWAGGER_CMD) generate client -f $(SPEC_FILE) -t ../../
	@cd ../..
	@echo "+ $@"
	@go generate -x $(REPO)/mock

test:
	@echo "+ $@"
	@go test -test.short -race -v $(PKGS)

coverage:
	@echo "+ $@"
	@for pkg in $(PKGS); do \
	  go test -test.short -coverprofile="../../../$$pkg/coverage.txt" $${pkg} || exit 1; \
	done

test-full:
	@echo "+ $@"
	@go test -race $(PKGS)

get-tools:
	@echo "+ $@"
	@go get -u \
		github.com/golang/lint/golint \
		github.com/wfarner/blockcheck \
		github.com/rancher/trash

# Set an output prefix, which is the local directory if not specified
PREFIX?=$(shell pwd -L)

# Used to populate version variable in main package.
VERSION=$(shell git describe --match 'v[0-9]*' --dirty='.m' --always)

# Allow turning off function inlining and variable registerization
ifeq (${DISABLE_OPTIMIZATION},true)
	GO_GCFLAGS=-gcflags "-N -l"
	VERSION:="$(VERSION)-noopt"
endif

.PHONY: clean all fmt vet lint build test binaries
.DEFAULT: all
all: fmt vet lint build test binaries

AUTHORS: .mailmap .git/HEAD
	 git log --format='%aN <%aE>' | sort -fu > $@

# Package list
PKGS := $(shell go list -tags "${MY_BUILDTAGS}" ./... | \
grep -v ^github.com/docker/libmachete/vendor/ | \
grep -v ^github.com/docker/libmachete/e2e/)

# A build target
${PREFIX}/bin/WHATEVER: $(wildcard **/*.go)
	@echo "+ $@"
	@go build -tags "${MY_BUILDTAGS}" -o $@ ${GO_LDFLAGS}  ${GO_GCFLAGS} ./cmd/WHATEVER

vet:
	@echo "+ $@"
	@go vet -tags "${MY_BUILDTAGS}" $(PKGS)

fmt:
	@echo "+ $@"
	@test -z "$$(gofmt -s -l . 2>&1 | grep -v ^vendor/ | tee /dev/stderr)" || \
		(echo >&2 "+ please format Go code with 'gofmt -s'" && false)

lint:
	@echo "+ $@"
	$(if $(shell which golint || echo ''), , \
		$(error Please install golint: `go get -u github.com/golang/lint/golint`))
	@test -z "$$(golint ./... 2>&1 | grep -v ^vendor/ | tee /dev/stderr)"

build:
	@echo "+ $@"
	@go build -tags "${MY_BUILDTAGS}" -v ${GO_LDFLAGS} $(PKGS)

test:
	@echo "+ $@"
	@go test -test.short -race -check.vv -tags "${MY_BUILDTAGS}" $(PKGS)

test-full:
	@echo "+ $@"
	@go test -check.vv -race -tags "${MY_BUILDTAGS}" $(PKGS)

# Godep helpers
dep-check-godep:
	$(if $(shell which godep || echo ''), , \
		$(error Please install godep: go get github.com/tools/godep))

dep-save: dep-check-godep
	@echo "+ $@"
	@godep save $(PKGS)

dep-restore: dep-check-godep
	@echo "+ $@"
	@godep restore -v

dep-validate: dep-restore
	@echo "+ $@"
	@rm -Rf .vendor.bak
	@mv vendor .vendor.bak
	@rm -Rf Godeps
	@godep save ./...
	@test -z "$$(diff -r vendor .vendor.bak 2>&1 | tee /dev/stderr)" || \
		(echo >&2 "+ borked dependencies! what you have in Godeps/Godeps.json does not match with what you have in vendor" && false)
	@rm -Rf .vendor.bak

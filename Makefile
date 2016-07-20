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

ci: fmt vet lint dep-validate coverage

AUTHORS: .mailmap .git/HEAD
	 git log --format='%aN <%aE>' | sort -fu > $@

# Package list
PKGS_AND_MOCKS := $(shell go list ./... | grep -v ^github.com/docker/libmachete/vendor/)
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

build:
	@echo "+ $@"
	@go build ${GO_LDFLAGS} $(PKGS)

install:
	@echo "+ $@"
	@go install ${GO_LDFLAGS} $(PKGS)

generate:
	@echo "+ $@"
	@go generate -x $(PKGS_AND_MOCKS)

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

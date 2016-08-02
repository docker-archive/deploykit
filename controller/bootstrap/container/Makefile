VERSION?=0.0-snapshot
REVISION=$(shell git rev-list -1 HEAD)

DOCKER_REPO?="docker4x/bootstrap"
DOCKER_TAG?="aws-v1.12.0-beta4"
DOCKER_IMAGE=$(DOCKER_REPO):$(DOCKER_TAG)

.PHONY: container

container:
	@echo "+ $@"
	cd ../../.. && LDFLAGS="-X main.Version=$(VERSION) -X main.Revision=$(REVISION)" \
	  ./scripts/build-in-docker controller/bootstrap/main.go controller/bootstrap/container/bin/bootstrap
	docker build -t $(DOCKER_IMAGE) -t $(DOCKER_REPO):latest -t $(DOCKER_REPO):$(VERSION) .

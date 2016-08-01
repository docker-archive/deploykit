VERSION?=0.0-snapshot
REVISION=$(shell git rev-list -1 HEAD)

DOCKER_REPO?="docker4x/bootstrap"
DOCKER_TAG?="aws-v1.12.0-beta4"
DOCKER_IMAGE=$(DOCKER_REPO):$(DOCKER_TAG)

.PHONY: container

container: binary
	@echo "+ $@"
	docker build -t $(DOCKER_IMAGE) -t $(DOCKER_REPO):latest -t $(DOCKER_REPO):$(VERSION) .

binary:
	@echo "+ $@"
	mkdir -p bin && CGO_ENABLED=0 GOOS=linux go build \
	  -ldflags "-X main.Version=$(VERSION) -X main.Revision=$(REVISION)" \
	  -o bin/bootstrap ../main.go

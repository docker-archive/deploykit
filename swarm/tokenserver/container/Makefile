VERSION?=0.0-snapshot
REVISION=$(shell git rev-list -1 HEAD)

DOCKER_REPO?="docker4x/tokenserver"
DOCKER_TAG?="dev"
DOCKER_IMAGE=$(DOCKER_REPO):$(DOCKER_TAG)

.PHONY: clean container

clean:
	@echo "+ $@"
	rm -rf bin

container: clean
	@echo "+ $@"
	mkdir -p bin
	cd ../../.. && LDFLAGS="-X main.Version=$(VERSION) -X main.Revision=$(REVISION)" \
	  ./scripts/build-in-docker swarm/tokenserver/main.go swarm/tokenserver/container/bin/tokenserver
	docker build -t $(DOCKER_IMAGE) -t $(DOCKER_REPO):latest -t $(DOCKER_REPO):$(VERSION) .

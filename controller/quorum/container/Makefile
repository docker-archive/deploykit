VERSION?=0.0-snapshot
REVISION=$(shell git rev-list -1 HEAD)

DOCKER_REPO_OWNER?=libmachete
DOCKER_REPO?=${DOCKER_REPO_OWNER}/quorum
DOCKER_TAG?=dev
DOCKER_IMAGE=${DOCKER_REPO}:${DOCKER_TAG}

.PHONY: container

clean:
	@echo "+ $@"
	rm -rf bin

container:
	@echo "+ $@"
	mkdir -p bin
	cd ../../.. && LDFLAGS="-X main.Version=$(VERSION) -X main.Revision=$(REVISION)" \
	  ./scripts/build-in-docker controller/quorum/quorum/main.go controller/quorum/container/bin/quorum
	docker build -t $(DOCKER_IMAGE) -t $(DOCKER_REPO):latest -t $(DOCKER_REPO):$(VERSION) .
ifeq (${DOCKER_PUSH},true)
	docker push ${DOCKER_IMAGE}
ifeq (${DOCKER_TAG_LATEST},true)
	docker push ${DOCKER_REPO}:latest
endif
endif

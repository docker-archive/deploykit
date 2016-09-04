# Makefile for building the watcher controller
#
# We use a container to compile the binary, and in
# the container we use govendor to fetch all the versioned
# packages and dependencies.
# If you need to change the version of a package, update
# the vendor/vendor.json and the build container will
# automatically pull in the versioned dependencies without
# changing your local workspace.

# Build version metadata
VERSION?=0.0-snapshot
REVISION=$(shell git rev-list -1 HEAD)

DOCKER_REPO_OWNER?=chungers
DOCKER_REPO?=${DOCKER_REPO_OWNER}/hello
DOCKER_TAG?=dev
DOCKER_IMAGE=${DOCKER_REPO}:${DOCKER_TAG}

.PHONY: clean all container
.DEFAULT: all
all: container

clean:
	@echo "+ $@"
	rm -rf bin
	@# Remove local images
	docker rmi -f ${DOCKER_IMAGE} >/dev/null 2>&1 || true
	docker rmi -f ${DOCKER_REPO}:latest >/dev/null 2>&1 || true

container: clean
	@echo "+ $@"
	mkdir -p bin
	cd ../../.. && LDFLAGS="-X main.Version=$(VERSION) -X main.Revision=$(REVISION)" \
	  ./scripts/build-in-docker 'controller/hello/cmd/*.go' controller/hello/container/bin/hello
	docker build -t $(DOCKER_IMAGE) -t $(DOCKER_REPO):latest -t $(DOCKER_REPO):$(VERSION) .

ifeq (${DOCKER_PUSH},true)
	docker push ${DOCKER_IMAGE}
ifeq (${DOCKER_TAG_LATEST},true)
	docker push ${DOCKER_REPO}:latest
endif
endif

# Must run on a clean linux host -- pinata does not expose /var/lib/docker/plugins
PLUGIN_ID?="hello-plugin-instance"
plugin: container
	@echo "making rootfs"
	docker export $(docker run ${DOCKER_REPO} -h) --output /var/lib/docker/plugins/$(PLUGIN_ID)/rootfs.tar
	tar xvf /var/lib/docker/plugins/$(PLUGIN_ID)/rootfs.tar -C /var/lib/docker/plugins/$(PLUGIN_ID)/rootfs/
	cp ./manifest.json  /var/lib/docker/plugins/$(PLUGIN_ID)/
	cp ./plugin-config.json  /var/lib/docker/plugins/$(PLUGIN_ID)/
	sed -e 's/@PLUGIN_ID@/$$PLUGIN_ID/g' ./plugins.json /var/lib/docker/plugins/
	@echo "stop docker"
	stop docker  # on ubuntu / upstart
	start docker # on ubuntu / upstart
	docker plugin ls
	docker plugin push $(DOCKER_REPO)-plugin:latest


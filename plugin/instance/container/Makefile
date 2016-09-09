# Makefile for building the instance plugin controller
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

PLUGIN?=aws-instance
PLUGIN_TYPE?=instanceDriver


DOCKER_REPO_OWNER?=chungers
DOCKER_TAG?=dev
PLUGIN_TAG?=latest

# Derived values

DOCKER_REPO=${DOCKER_REPO_OWNER}/${PLUGIN}-c
DOCKER_IMAGE=${DOCKER_REPO}:${DOCKER_TAG}

PLUGIN_REPO_OWNER?=${DOCKER_REPO_OWNER}
PLUGIN_REPO=${PLUGIN}
PLUGIN_IMAGE=${PLUGIN_REPO}:${PLUGIN_TAG}

PLUGIN_NAME=${PLUGIN_REPO_OWNER}/${PLUGIN_REPO}
PLUGIN_ID=${PLUGIN}-id


.PHONY: clean all container
.DEFAULT: all
all: container

clean:
	@echo "+ $@"
	rm -rf bin
	@# Remove local images
	docker rmi -f ${DOCKER_IMAGE} >/dev/null 2>&1 || true
	docker rmi -f ${DOCKER_REPO}:latest >/dev/null 2>&1 || true

# Container is necessary to set up the rootfs of the plugin.
# Plugins however are not as flexible as containers.... we need to have different versions of plugins for
# different infrastructure providers.  However the different versions of the plugin will share the same
# binary, just different entry point (e.g. the AWS plugin will specify 'aws' in the plugin manifest as command.
container: clean
	@echo "+ $@ GOPATH=${GOPATH}"
	mkdir -p bin
	cd ../../.. && LDFLAGS="-X main.Version=$(VERSION) -X main.Revision=$(REVISION) -X main.PluginName=$(PLUGIN_NAME)" \
	  ./hack/build-in-docker 'plugin/instance/cmd/*.go' plugin/instance/container/bin/driver
	docker build -t $(DOCKER_IMAGE) -t $(DOCKER_REPO):latest -t $(DOCKER_REPO):$(VERSION) .

ifeq (${DOCKER_PUSH},true)
	docker push ${DOCKER_IMAGE}
ifeq (${DOCKER_TAG_LATEST},true)
	docker push ${DOCKER_REPO}:latest
endif
endif


SSH?=ubuntu@54.67.4.42

# Must run on a clean linux host -- pinata does not expose /var/lib/docker/plugins
# Note the manifest.json still has the name (repo name) of the plugin. So this is not totally parameterized.
plugin: container
	-ssh ${SSH} 'sudo docker ps -aq | xargs docker rm'
	-ssh ${SSH} sudo docker rmi ${DOCKER_REPO}
	@echo "making rootfs"
	ssh ${SSH} sudo docker run ${DOCKER_REPO} -h
	@echo "stopping docker"
	ssh ${SSH} sudo stop docker
	ssh ${SSH} sudo rm -rf /var/lib/docker/plugins/${PLUGIN_ID}
	ssh ${SSH} sudo mkdir -p /var/lib/docker/plugins/${PLUGIN_ID}/rootfs/
	ssh ${SSH} sudo mkdir -p /var/lib/docker/plugins/${PLUGIN_ID}/rootfs/run/docker/plugins
	@echo "starting docker"
	ssh ${SSH} sudo start docker
	ssh ${SSH} "sudo docker ps -aq | head -1 | xargs sudo docker export --output /var/lib/docker/plugins/${PLUGIN_ID}/rootfs.tar"
	ssh ${SSH} "sudo tar xvf /var/lib/docker/plugins/${PLUGIN_ID}/rootfs.tar -C /var/lib/docker/plugins/${PLUGIN_ID}/rootfs/"
	@echo "copying manifest.json"
	scp ./manifest.json ${SSH}:/tmp/manifest.json
	ssh ${SSH} sudo cp /tmp/manifest.json /var/lib/docker/plugins/${PLUGIN_ID}/manifest.json
	@echo "copying plugin-config.json"
	scp ./plugin-config.json ${SSH}:/tmp/plugin-config.json
	ssh ${SSH} sudo cp /tmp/plugin-config.json /var/lib/docker/plugins/${PLUGIN_ID}/plugin-config.json
	@echo "copying plugins.json"
	sed -e 's/@PLUGIN_ID@/${PLUGIN_ID}/g' \
	    -e 's/@PLUGIN_REPO_OWNER@/${PLUGIN_REPO_OWNER}/g' \
	    -e 's/@PLUGIN_REPO@/${PLUGIN_REPO}/g' \
	    ./plugins.json > /tmp/plugins.json
	scp /tmp/plugins.json ${SSH}:/tmp/plugins.json
	ssh ${SSH} sudo cp /tmp/plugins.json /var/lib/docker/plugins/plugins.json
	@echo "stopping docker"
	ssh ${SSH} sudo stop docker
	@echo "starting docker backup"
	ssh ${SSH} sudo start docker
	@echo "checking docker log"
	ssh ${SSH} sudo cat /var/log/upstart/docker.log
	@echo "checking plugin"
	ssh ${SSH} sudo docker plugin ls
	@echo "push the plugin from the host"

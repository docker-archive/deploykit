FROM golang:1.10.0-alpine3.7
RUN apk add --update git make gcc musl-dev wget ca-certificates openssl libvirt-dev libvirt-static libvirt-lxc libvirt-qemu git openssh file
ENV GOPATH /go
ENV PATH /go/bin:$PATH
COPY dockerfiles/build-infrakit /usr/local/bin/
# Add source code
Add . /go/src/github.com/docker/infrakit/
WORKDIR /go/src/github.com/docker/infrakit
RUN mkdir ./build && make binaries


FROM alpine:latest
RUN apk add --update wget ca-certificates openssl libvirt-dev libvirt-static openssh file
# server-side dirs
RUN mkdir -p /.infrakit/plugins /.infrakit/configs /.infrakit/logs /.infrakit/instance/terraform
# Default single node leader file
RUN echo manager1 > /.infrakit/leader
VOLUME /.infrakit
WORKDIR /.infrakit
ENV INFRAKIT_HOME /.infrakit
# Defined in pkg/discovery
ENV INFRAKIT_PLUGINS_DIR /.infrakit/plugins
# When using the manager 'os' option
ENV INFRAKIT_LEADER_FILE /.infrakit/leader
ENV INFRAKIT_STORE_DIR /.infrakit/configs
# client-side dirs
RUN mkdir -p /.infrakit-session/playbook-cache /.infrakit-session/cli
# Defined in pkg/cli
ENV INFRAKIT_CLI_DIR /.infrakit-session/cli
# Defined in pkg/cli
ENV INFRAKIT_CLI_DIR /.infrakit-session/cli
# Defined in cmd/.infrakit/playbook
ENV INFRAKIT_PLAYBOOKS_FILE /.infrakit-session/playbooks.yml
ENV INFRAKIT_PLAYBOOKS_CACHE /.infrakit-session/playbook-cache
COPY --from=0 /go/src/github.com/docker/infrakit/build/* /usr/local/bin/

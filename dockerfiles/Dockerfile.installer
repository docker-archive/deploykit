FROM golang:1.10.0-alpine3.7

RUN apk add --update git make gcc musl-dev wget ca-certificates openssl libvirt-dev libvirt-static libvirt-lxc libvirt-qemu git openssh file

ENV GOPATH /go
ENV PATH /go/bin:$PATH

COPY dockerfiles/build-infrakit /usr/local/bin/

# Add source code
Add . /go/src/github.com/docker/infrakit/

RUN mkdir /build
VOLUME /build

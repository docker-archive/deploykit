FROM golang:1.10.0-alpine3.7

RUN apk add --update git make gcc musl-dev wget ca-certificates openssl libvirt-dev libvirt-static

WORKDIR /go/src/github.com/docker/infrakit

VOLUME [ "/go/src/github.com/docker/infrakit/build" ]

COPY . ./

CMD make infrakit-linux terraform-linux

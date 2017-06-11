FROM golang:1.7.4-alpine

# A container for building InfraKit

RUN apk add --update git make

ENV CGO_ENABLED=0
ENV GOPATH=/go
ENV PATH=/go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# Development tools
RUN go get github.com/rancher/trash \
           github.com/golang/lint/golint \
           github.com/golang/mock/gomock \
           github.com/golang/mock/mockgen

# The project sources
ADD ./ /go/src/github.com/docker/infrakit.aws
WORKDIR /go/src/github.com/docker/infrakit.aws

VOLUME [ "/go/src/github.com/docker/infrakit.aws/build" ]

RUN trash  # Force updating the vendored sources per spec; this slows the build but is most correct.

CMD make build-binaries

FROM golang:1.8.0-alpine

RUN apk add --update git make

WORKDIR /go/src/github.com/docker/infrakit/pkg/provider/google
VOLUME ["/go/src/github.com/docker/infrakit/pkg/provider/google/build"]
CMD ["make", "build-binaries"]

COPY . ./

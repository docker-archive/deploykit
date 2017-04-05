FROM golang:1.8.0-alpine

RUN apk add --update git make

WORKDIR /go/src/github.com/docker/infrakit.gcp
VOLUME ["/go/src/github.com/docker/infrakit.gcp/build"]
CMD ["make", "build-binaries"]

COPY . ./

FROM golang:1.7.3-alpine

RUN apk update && apk add --upgrade ca-certificates git make

ENV CGO_ENABLED=0
ENV GOPATH=/go

ADD build.sh /build.sh

CMD /build.sh

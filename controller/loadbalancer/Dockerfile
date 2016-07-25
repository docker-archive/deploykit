FROM alpine:3.3

MAINTAINER David Chung <david.chung@docker.com>

RUN apk add --update ca-certificates && rm -Rf /tmp/* /var/lib/cache/apk/*

# needed in order for go binary to work.
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

ADD build/controller /usr/bin/

ENTRYPOINT [ "loadbalancer" ]

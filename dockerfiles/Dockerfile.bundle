FROM alpine:3.5

RUN apk add --update ca-certificates

RUN mkdir -p /infrakit/plugins /infrakit/configs /infrakit/logs

VOLUME /infrakit

ENV INFRAKIT_HOME /infrakit
ENV INFRAKIT_PLUGINS_DIR /infrakit/plugins

ADD build/* /usr/local/bin/

# Make symbolic links to make standardized bin names.
# This makes for shorter names when containers are already scoped by the platform (eg. infrakit/aws)
RUN ln -s /usr/local/bin/infrakit-instance-aws /usr/bin/instance
RUN ln -s /usr/local/bin/infrakit-metadata-aws /usr/bin/metadata

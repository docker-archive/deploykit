FROM alpine:3.5

RUN apk add --update ca-certificates

RUN mkdir -p /infrakit/plugins /infrakit/configs /infrakit/logs

VOLUME /infrakit
VOLUME /infrakit/platforms/gcp/credentials.json

ENV INFRAKIT_HOME /infrakit
ENV INFRAKIT_PLUGINS_DIR /infrakit/plugins

# For Google auth.  Be sure to bind mount the actual file to this location:
ENV GOOGLE_APPLICATION_CREDENTIALS /infrakit/platforms/gcp/credentials.json

ADD build/infrakit-instance-gcp /usr/local/bin/

# Make symbolic links to make standardized bin names.
# This makes for shorter names when containers are already scoped by the platform (eg. infrakit/gcp)
RUN ln -s /usr/local/bin/infrakit-instance-gcp /usr/bin/instance

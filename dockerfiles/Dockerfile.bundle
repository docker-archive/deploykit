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

# Default terraform directory
ENV INFRAKIT_INSTANCE_TERRAFORM_DIR /.infrakit/instance/terraform

#########################
# Client-side set up

# client-side dirs
RUN mkdir -p /.infrakit-session/playbook-cache /.infrakit-session/cli

# Defined in pkg/cli
ENV INFRAKIT_CLI_DIR /.infrakit-session/cli

# Defined in cmd/.infrakit/playbook
ENV INFRAKIT_PLAYBOOKS_FILE /.infrakit-session/playbooks.yml
ENV INFRAKIT_PLAYBOOKS_CACHE /.infrakit-session/playbook-cache

ADD build/* /usr/local/bin/

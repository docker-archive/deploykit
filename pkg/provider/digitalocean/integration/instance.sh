#!/usr/bin/env bash

set -e

BASEDIR=$(dirname "$0")
INFRAKIT_IMAGE=infrakit/devbundle:master-1580
TAG="ci-infrakit-digitalocean-instance-${CIRCLE_BUILD_NUM:-local}"
docker_run="docker run -v infrakit:/infrakit -e INFRAKIT_PLUGINS_DIR=/infrakit"


cleanup() {
  echo Clean up

  docker rm -f flavor group instance-digitalocean 2>/dev/null || true
  docker volume rm infrakit 2>/dev/null || true
}

remove_previous_instances() {
  echo Remove previous instances

  # FIXME(vdemeester) use doctl here
}


run_infrakit() {
  echo Run Infrakit

  docker volume create --name infrakit

  $docker_run -d --name=flavor ${INFRAKIT_IMAGE} infrakit-flavor-vanilla --log=5
  $docker_run -d --name=group ${INFRAKIT_IMAGE} infrakit-group-default --log=5
}

build_infrakit_digitalocean() {
  echo Build Infrakit DigitalOcean Instance Plugin

  pushd ..
  DOCKER_PUSH=false DOCKER_TAG_LATEST=false DOCKER_BUILD_FLAGS="" make build-docker
  popd
}

run_infrakit_digitalocean_instance() {
  echo Run Infrakit DigitalOcean Instance Plugin

  # FIXME(vdemeester) pass the token here !
  $docker_run -d --name=instance-digitalocean \
    infrakit/digitalocean:dev \
    infrakit-instance-digitalocean --log=5 --region=ams2 --access-token=${DO_TOKEN}
}

create_group() {
  echo Create Instance Group

  docker cp ${BASEDIR}/instances.json group:/infrakit/

  $docker_run --rm busybox sed -i.bak s/{{TAG}}/${TAG}/g /infrakit/instances.json
  $docker_run --rm busybox cat /infrakit/instances.json
  $docker_run --rm ${INFRAKIT_IMAGE} infrakit group commit /infrakit/instances.json
}

check_instances_created() {
  echo Check that the instances are there

  # TODO(vdemeester) use doctl for this
}

check_instance_properties() {
  echo Check that the instances are well configured

  # TODO(vdemeester) use doctl for this
}

delete_instances() {
  echo Delete instances

  # TODO(vdemeester) use doctl for this
}

destroy_group() {
  echo Destroy Instance Group

  $docker_run --rm ${INFRAKIT_IMAGE} infrakit group destroy instances
}

check_instances_gone() {
  echo Check that the instances are gone

  COUNT=0

  # TODO(vdemeester) use doctl for this

  echo "ERROR: ${COUNT} instances are still around"
  exit 1
}

assert_contains() {
  STDIN=$(cat)
  echo "${STDIN}" | grep -q "${1}" || (echo "Expected [${STDIN}] to contain [${1}]" && return 1)
}

assert_equals() {
  STDIN=$(cat)
  [ "${STDIN}" == "${1}" ] || (echo "Expected [${1}], got [${STDIN}]" && return 1)
}

[ -n "${DO_TOKEN}" ] || exit 1
cleanup
remove_previous_instances
run_infrakit
build_infrakit_digitalocean
run_infrakit_digitalocean_instance
create_group
check_instances_created
check_instance_properties
delete_instances
check_instances_created
check_instance_properties
destroy_group
check_instances_gone
cleanup

exit 0

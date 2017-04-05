#!/bin/bash

set -e

BASEDIR=$(dirname "$0")
INFRAKIT_IMAGE=infrakit/devbundle:master-1580
GCLOUD="docker run -e CLOUDSDK_CORE_PROJECT --rm -v gcloud-config:/.config google/cloud-sdk gcloud"
TAG="ci-infrakit-gcp-instance-${CIRCLE_BUILD_NUM:-local}"
docker_run="docker run -v infrakit:/infrakit -e INFRAKIT_PLUGINS_DIR=/infrakit"

export CLOUDSDK_CORE_PROJECT="${CLOUDSDK_CORE_PROJECT:-docker4x}"
export CLOUDSDK_COMPUTE_ZONE="${CLOUDSDK_COMPUTE_ZONE:-us-central1-f}"

cleanup() {
  echo Clean up

  docker rm -f flavor group instance-gcp 2>/dev/null || true
  docker volume rm infrakit 2>/dev/null || true
  docker volume rm gcloud-config 2>/dev/null || true
}

auth_gcloud() {
  echo Authenticate GCloud

  docker volume create --name gcloud-config
  docker run --rm -e GCLOUD_SERVICE_KEY -v gcloud-config:/.config google/cloud-sdk bash -c 'echo ${GCLOUD_SERVICE_KEY} | base64 --decode > /.config/key.json'
  docker run --rm -v gcloud-config:/.config google/cloud-sdk gcloud auth activate-service-account --key-file=/.config/key.json
}

remove_previous_instances() {
  echo Remove previous instances

  OLD=$(${GCLOUD} compute instances list --filter="tags.items:${TAG}" --uri)
  if [ -n "${OLD}" ]; then
    ${GCLOUD} compute instances delete -q --delete-disks=boot ${OLD}
  fi
}

run_infrakit() {
  echo Run Infrakit

  docker volume create --name infrakit

  $docker_run -d --name=flavor ${INFRAKIT_IMAGE} infrakit-flavor-vanilla --log=5
  $docker_run -d --name=group ${INFRAKIT_IMAGE} infrakit-group-default --log=5
}

build_infrakit_gcp() {
  echo Build Infrakit GCP Instance Plugin

  pushd ..
  DOCKER_PUSH=false DOCKER_TAG_LATEST=false DOCKER_BUILD_FLAGS="" make build-docker
  popd
}

run_infrakit_gcp_instance() {
  echo Run Infrakit GCP Instance Plugin

  $docker_run -d --name=instance-gcp \
    -v gcloud-config:/.config \
    -e CLOUDSDK_CORE_PROJECT \
    -e CLOUDSDK_COMPUTE_ZONE \
    -e GOOGLE_APPLICATION_CREDENTIALS=/.config/key.json \
    infrakit/gcp:dev \
    infrakit-instance-gcp --log=5
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

  for i in $(seq 1 120); do
    COUNT=$(${GCLOUD} compute instances list --filter="status:RUNNING AND tags.items:${TAG}" --uri | wc -w | tr -d '[:space:]')
    echo "- ${COUNT} instances were created"

    if [ ${COUNT} -gt 2 ]; then
      echo "- ERROR: that's too many!"
      exit 1
    fi

    if [ ${COUNT} -eq 2 ]; then
      return
    fi

    docker logs --tail 1 group
    sleep 1
  done
}

check_instance_properties() {
  echo Check that the instances are well configured

  URIS=$(${GCLOUD} compute instances list --filter="status:RUNNING AND tags.items:${TAG}" --uri)
  for URI in ${URIS}; do
    echo - Check ${URI}

    JSON=$(${GCLOUD} compute instances describe ${URI} --format=json)

    echo "  - Check description"
    echo "${JSON}" | jq -r '.description' | assert_equals "Test of GCP infrakit"

    echo "  - Check zone"
    echo "${JSON}" | jq -r '.zone' | assert_contains "/projects/${CLOUDSDK_CORE_PROJECT}/zones/${CLOUDSDK_COMPUTE_ZONE}"

    echo "  - Check network"
    echo "${JSON}" | jq -r '.networkInterfaces[0].network' | assert_contains "/projects/${CLOUDSDK_CORE_PROJECT}/global/networks/default"

    echo "  - Check machine type"
    echo "${JSON}" | jq -r '.machineType' | assert_contains "/projects/${CLOUDSDK_CORE_PROJECT}/zones/${CLOUDSDK_COMPUTE_ZONE}/machineTypes/f1-micro"

    echo "  - Check name"
    echo "${JSON}" | jq -r '.name' | assert_contains "test-"

    echo "  - Check tags"
    echo "${JSON}" | jq -r '.tags.items[]' | assert_equals "${TAG}"

    echo "  - Check disk type"
    echo "${JSON}" | jq -r '.disks[0].type' | assert_equals "PERSISTENT"

    echo "  - Check startup script"
    ${GCLOUD} compute instances get-serial-port-output ${URI} 2>/dev/null | assert_contains "Hello, World"
  done
}

delete_instances() {
  echo Delete instances

  OLD=$(${GCLOUD} compute instances list --filter="tags.items:${TAG}" --uri)
  if [ -n "${OLD}" ]; then
    ${GCLOUD} compute instances delete -q ${OLD}
  fi
}

destroy_group() {
  echo Destroy Instance Group

  $docker_run --rm ${INFRAKIT_IMAGE} infrakit group destroy instances
}

check_instances_gone() {
  echo Check that the instances are gone

  COUNT=$(${GCLOUD} compute instances list --filter="tags.items:${TAG}" --uri | wc -w | tr -d '[:space:]')
  if [ ${COUNT} -eq 0 ]; then
    echo "- All instances are gone"
    return
  fi

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

[ -n "${GCLOUD_SERVICE_KEY}" ] || exit 1
cleanup
auth_gcloud
remove_previous_instances
run_infrakit
build_infrakit_gcp
run_infrakit_gcp_instance
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

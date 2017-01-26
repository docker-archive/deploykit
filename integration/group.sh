#!/bin/bash

set -e

BASEDIR=$(dirname "$0")
INFRAKIT_IMAGE=infrakit/devbundle:master-1131
GCLOUD="docker run -e CLOUDSDK_CORE_PROJECT --rm -v gcloud-config:/.config google/cloud-sdk gcloud"
TAG="ci-infrakit-gcp-group-${CIRCLE_BUILD_NUM:-local}"

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

  OLD=$(${GCLOUD} compute instance-groups managed list --filter="name:${TAG}" --uri)
  if [ -n "${OLD}" ]; then
    ${GCLOUD} compute instance-groups managed delete --zone=${CLOUDSDK_COMPUTE_ZONE} -q ${OLD}
  fi

  OLD=$(${GCLOUD} compute instance-templates list --filter="name:${TAG}" --uri)
  if [ -n "${OLD}" ]; then
    ${GCLOUD} compute instance-templates delete -q ${OLD}
  fi

  OLD=$(${GCLOUD} compute instances list --filter="tags.items:${TAG}" --uri)
  if [ -n "${OLD}" ]; then
    ${GCLOUD} compute instances delete -q --delete-disks=boot ${OLD}
  fi
}

run_infrakit() {
  echo Run Infrakit

  docker volume create --name infrakit

  run_plugin='docker run -d -v infrakit:/root/.infrakit/'

  $run_plugin --name=flavor ${INFRAKIT_IMAGE} infrakit-flavor-vanilla --log=5
}

build_infrakit_gcp() {
  echo Build Infrakit GCP Instance Plugin

  pushd ..
  DOCKER_PUSH=false DOCKER_TAG_LATEST=false DOCKER_BUILD_FLAGS="" make build-docker
  popd
}

run_infrakit_gcp_group() {
  echo Run Infrakit GCP Group Plugin

  docker run -d --name=group \
    -v infrakit:/root/.infrakit/ \
    -v gcloud-config:/.config \
    -e CLOUDSDK_CORE_PROJECT \
    -e CLOUDSDK_COMPUTE_ZONE \
    -e GOOGLE_APPLICATION_CREDENTIALS=/.config/key.json \
    infrakit/gcp:dev \
    infrakit-group-gcp --log=5 --name=group
}

create_group() {
  echo Create Instance Group

  docker_run="docker run --rm -v infrakit:/root/.infrakit/"
  docker cp ${BASEDIR}/group.json group:/root/.infrakit/
  $docker_run busybox sed -i.bak s/{{TAG}}/${TAG}/g /root/.infrakit/group.json
  $docker_run busybox cat /root/.infrakit/group.json
  $docker_run ${INFRAKIT_IMAGE} infrakit group commit /root/.infrakit/group.json
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

destroy_group() {
  echo Destroy Instance Group

  docker run --rm -v infrakit:/root/.infrakit/ ${INFRAKIT_IMAGE} infrakit group destroy ${TAG}
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
run_infrakit_gcp_group
create_group
check_instances_created
check_instance_properties
destroy_group
check_instances_gone
cleanup

exit 0

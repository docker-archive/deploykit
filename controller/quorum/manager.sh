#!/bin/bash
#
# This is a script intended to be injected into boot-time 'user data' for a Docker Swarm manager instance.

install_script() {
  set -o errexit
  set -o nounset
  set -o pipefail

  install_script_src="$1"

  sleep 5
  curl -sSL https://get.docker.com/ | sh

  CLUSTER_NAME=machete-testing
  MACHETE_PORT=8888
  DRIVER_AND_ARGS='aws --region us-west-2'
  MANAGER_IPS=10.78.190.2,10.78.190.3,10.78.190.4
  BOOT_LEADER=$(echo $MANAGER_IPS | tr ',' '\n' | head -n1)
  NUM_WORKERS=3

  boot_args="join $BOOT_LEADER"
  if [[ $(hostname -i) = "$BOOT_LEADER" ]]
  then
    echo 'This is the leader, initializing cluster'
    boot_args="init"
  fi

  # Coordinates swarm boot sequence.
  docker run \
    --detach \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    wfarner/swarmboot \
    $boot_args

  # Machete API server.
  docker run \
    --detach \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    --restart always \
    wfarner/machete \
    --cluster "$CLUSTER_NAME" --port $MACHETE_PORT $DRIVER_AND_ARGS

  # Serves swarm join tokens.
  docker run \
    --detach \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    --restart always \
    --publish 8889:8889 \
    wfarner/tokenserver \
    run

  manager_setup_script="#!/bin/bash
$install_script_src

install_script_src=\"\$\(declare -f install_script\)\"
install_script \"\$install_script_src\" &
"
  manager_user_data=$(echo "$manager_setup_script" | base64)

  cat << EOF > ./manager-request.swpt
{
    "group": "managers",
    "tags": {"bill-machete-testing": "testing"},
    "run_instances_input": {
        "BlockDeviceMappings": [
          {
            "DeviceName": "/dev/sdb",
            "Ebs": {
                "DeleteOnTermination": true,
                "VolumeSize": 64,
                "VolumeType": "gp2"
            }
          }
        ],
        "EbsOptimized": false,
        "ImageId": "ami-b9ff39d9",
        "InstanceType": "t2.small",
        "KeyName": "dev",
        "PrivateIpAddress": "{{.IP}}",
        "Placement": {
            "AvailabilityZone": "us-west-2c"
        },
        "SubnetId": "subnet-d270878a",
        "SecurityGroupIds": ["sg-89f23fef"],
        "UserData": "$manager_user_data"
    }
}
EOF

  # Maintains manager node pool (managers 'watch' each other).
  docker run \
    --detach \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    --restart always \
    wfarner/quorum \
    localhost:$MACHETE_PORT $MANAGER_IPS ./manager-request.swpt


  worker_setup_script="#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

start_install() {
  sleep 5
  curl -sSL https://get.docker.com/ | sh
  docker run \
    --detach \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    wfarner/swarmboot \
    join $BOOT_LEADER --worker
}

start_install &
"
  worker_user_data=$(echo "$worker_setup_script" | base64)

  cat << EOF > ./worker-request.swpt
{
    "group": "workers",
    "tags": {"bill-machete-testing": "testing"},
    "run_instances_input": {
        "BlockDeviceMappings": [
          {
            "DeviceName": "/dev/sdb",
            "Ebs": {
                "DeleteOnTermination": true,
                "VolumeSize": 64,
                "VolumeType": "gp2"
            }
          }
        ],
        "EbsOptimized": false,
        "ImageId": "ami-b9ff39d9",
        "InstanceType": "t2.small",
        "KeyName": "dev",
        "Placement": {
            "AvailabilityZone": "us-west-2c"
        },
        "SubnetId": "subnet-d270878a",
        "SecurityGroupIds": ["sg-89f23fef"],
        "UserData": "$worker_user_data"
    }
}
EOF

  # Maintains worker node pool.
  docker run \
    --detach \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    --restart always \
    wfarner/scaler \
    localhost:$MACHETE_PORT $NUM_WORKERS ./worker-request.swpt
}

# The install script function source is an input to itself to allow for 'self-replication' of manager nodes
# via the quorum controller.  For this reason, it's important to keep the majority of logic in the install_script
# function.
install_script_src=$(declare -f install_script)

# See https://github.com/docker/docker/issues/23793#issuecomment-237735835 for
# details on why we background/sleep.
install_script "$install_script_src" &

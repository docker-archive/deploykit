#!/bin/bash

set -o errexit
set -o nounset
set -o xtrace

# See http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/device_naming.html why device naming is tricky, and likely
# coupled to the AMI (host OS) used.
EBS_DEVICE=/dev/xvdf

# Check to see that we don't already have this set up
setup=$(grep '/var/lib/docker' /etc/fstab | awk '{print $1}')
if [ "${setup}" = "/dev/xvdf" ]; then
    echo "Skipping setup of volumes."
else

echo "Setting up volumes"

# TODO - make this more robust - loop and check.  Right now just sleeps and hope for the best.
if [ ! -b $EBS_DEVICE ]; then
    echo "Device $EBS_DEVICE not ready. Waiting"
    sleep 30
fi

# Determine whether the EBS volume needs to be formatted.
if [ "$(file -sL $EBS_DEVICE)" = "$EBS_DEVICE: data" ]
then
    echo 'Formatting EBS volume device'
    mkfs -t ext4 $EBS_DEVICE
fi

stopped=0
if [ -d "/var/lib/docker" ]; then
    service docker stop
    rm -rf /var/lib/docker
    stopped=1
fi

mkdir -p /var/lib/docker
echo "$EBS_DEVICE /var/lib/docker ext4 defaults,nofail 0 2" >> /etc/fstab

echo "Mounting /var/lib/docker"
mount -a

if [ "$stopped" -eq "1" ]; then
    echo "Starting Docker"
    service docker start
fi

fi # setup

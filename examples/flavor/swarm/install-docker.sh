
# Tested on Ubuntu/trusty

apt-get update -y
apt-get upgrade -y
wget -qO- https://get.docker.com/ | sh

# Tell Docker to listen on port 4243 for remote API access. This is optional.
echo DOCKER_OPTS=\"-H tcp://0.0.0.0:4243 -H unix:///var/run/docker.sock\" >> /etc/default/docker

# Restart Docker to let port listening take effect.
service docker restart

{{/* =% sh %= */}}

echo "Installing Hyperkit plugin on your Mac"

export INFRAKIT_HOME=~/.infrakit
mkdir -p $INFRAKIT_HOME/configs
mkdir -p $INFRAKIT_HOME/logs

docker run --rm -v `pwd`:/build infrakit/installer build-hyperkit

echo "Version of this hyperkit plugin:"
./infrakit-instance-hyperkit version

echo "Copying hyperkit plugin to /usr/local/bin"
sudo cp `pwd`/infrakit-instance-hyperkit /usr/local/bin/

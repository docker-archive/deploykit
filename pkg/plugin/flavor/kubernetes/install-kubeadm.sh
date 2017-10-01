apt-get install -y apt-transport-https curl
curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
cat <<EOF >/etc/apt/sources.list.d/kubernetes.list
deb http://apt.kubernetes.io/ kubernetes-xenial main
EOF
apt-get update
# Install docker if you don't have it already.
apt-get install -y\
    apt-transport-https \
    ca-certificates \
    software-properties-common \
    kubelet \
    kubeadm \
    kubectl \
    kubernetes-cni

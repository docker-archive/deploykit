#!/bin/bash

# On MacOS X
HostIP=$(ipconfig getifaddr en0)

# uses Docker container
docker run -d \
       -v /usr/share/ca-certificates/:/etc/ssl/certs \
       -p 14001:4001 \
       -p 12380:2380 \
       -p 12379:2379 \
       --name etcd0 \
       quay.io/coreos/etcd etcd \
       -name etcd0  \
       -listen-peer-urls http://0.0.0.0:2380  \
       -listen-client-urls http://0.0.0.0:2379,http://0.0.0.0:4001  \
       -advertise-client-urls http://${HostIP}:12379,http://${HostIP}:14001  \
       -initial-advertise-peer-urls http://${HostIP}:12380  \
       -initial-cluster-token etcd-cluster-1  \
       -initial-cluster etcd0=http://${HostIP}:12380,etcd1=http://${HostIP}:22380,etcd2=http://${HostIP}:32380  \
       -initial-cluster-state new

docker run -d \
       -v /usr/share/ca-certificates/:/etc/ssl/certs \
       -p 24001:4001 \
       -p 22380:2380 \
       -p 22379:2379 \
       --name etcd1 \
       quay.io/coreos/etcd etcd \
       -name etcd1  \
       -listen-peer-urls http://0.0.0.0:2380  \
       -listen-client-urls http://0.0.0.0:2379,http://0.0.0.0:4001  \
       -advertise-client-urls http://${HostIP}:22379,http://${HostIP}:24001  \
       -initial-advertise-peer-urls http://${HostIP}:22380  \
       -initial-cluster-token etcd-cluster-1  \
       -initial-cluster etcd0=http://${HostIP}:12380,etcd1=http://${HostIP}:22380,etcd2=http://${HostIP}:32380  \
       -initial-cluster-state new

docker run -d \
       -v /usr/share/ca-certificates/:/etc/ssl/certs \
       -p 34001:4001 \
       -p 32380:2380 \
       -p 32379:2379 \
       --name etcd2 \
       quay.io/coreos/etcd etcd \
       -name etcd2  \
       -listen-peer-urls http://0.0.0.0:2380  \
       -listen-client-urls http://0.0.0.0:2379,http://0.0.0.0:4001  \
       -advertise-client-urls http://${HostIP}:32379,http://${HostIP}:34001  \
       -initial-advertise-peer-urls http://${HostIP}:32380  \
       -initial-cluster-token etcd-cluster-1  \
       -initial-cluster etcd0=http://${HostIP}:12380,etcd1=http://${HostIP}:22380,etcd2=http://${HostIP}:32380  \
       -initial-cluster-state new



# quick test
docker run -ti -e ETCDCTL_API=3 quay.io/coreos/etcd  etcdctl -C http://${HostIP}:12379 member list

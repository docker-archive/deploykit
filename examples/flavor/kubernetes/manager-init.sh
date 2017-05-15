#!/bin/sh
set -o errexit
set -o nounset
set -o xtrace

{{/* Install Docker */}}
{{ include "install-docker.sh" }}

{{/* Install Kubeadm */}}
{{ include "install-kubeadm.sh" }}

kubeadm init --token {{ KUBEADM_JOIN_TOKEN }}
export KUBECONFIG=/etc/kubernetes/admin.conf
{{ if NETWORK_ADDON }}
    kubectl apply -f {{ NETWORK_ADDON }}
{{ else }}
{{ end }}

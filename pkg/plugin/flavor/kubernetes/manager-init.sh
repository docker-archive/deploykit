#!/bin/sh
set -o errexit
set -o nounset
set -o xtrace

{{/* Install Docker */}}
{{ include "install-docker.sh" }}

{{/* Install Kubeadm */}}
{{ include "install-kubeadm.sh" }}

{{ if BOOTSTRAP }}
# Bootstrap node -- init cluster and install add-ons
kubeadm init --token {{ KUBEADM_JOIN_TOKEN }}

export KUBECONFIG=/etc/kubernetes/admin.conf
{{ if ADDON "network" }}
    kubectl apply -f {{ ADDON "network" }}
{{ else }}
{{ end }}
{{ if ADDON "visualise" }}
    kubectl apply -f {{ ADDON "visualise" }}
{{ else }}
{{ end }}

{{ if WORKER }}
# Manager node but because we are doing a single master control plane, we will just reuse the node as worker
kubeadm join --token {{ KUBEADM_JOIN_TOKEN }} {{ KUBE_JOIN_IP }}:{{ BIND_PORT }}
{{ end }}

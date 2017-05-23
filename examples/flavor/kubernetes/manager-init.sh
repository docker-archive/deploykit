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
{{ if ADDON "network" }}
    kubectl apply -f {{ ADDON "network" }}
{{ else }}
{{ end }}
{{ if ADDON "visualise" }}
    kubectl apply -f {{ ADDON "visualise" }}
{{ else }}
{{ end }}

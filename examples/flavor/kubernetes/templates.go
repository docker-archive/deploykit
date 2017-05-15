package main

const (
	// DefaultManagerInitScriptTemplate is the default template for the init script which
	// the flavor injects into the user data of the instance to configure Docker Swarm Managers
	DefaultManagerInitScriptTemplate = `
#!/bin/sh
set -o errexit
set -o nounset
set -o xtrace

kubeadm init --token {{ KUBEADM_JOIN_TOKEN }}
export KUBECONFIG=/etc/kubernetes/admin.conf
{{ if NETWORK_ADDON }}
    kubectl apply -f {{ NETWORK_ADDON }}
{{ else }}
{{ end }}
`

	// DefaultWorkerInitScriptTemplate is the default template for the init script which
	// the flavor injects into the user data of the instance to configure Docker Swarm.
	DefaultWorkerInitScriptTemplate = `
#!/bin/sh
set -o errexit
set -o nounset
set -o xtrace

kubeadm join --token {{ KUBEADM_JOIN_TOKEN }} {{ KUBE_JOIN_IP }}:{{ BIND_PORT }}
`
)

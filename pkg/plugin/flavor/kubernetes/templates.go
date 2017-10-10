package kubernetes

const (
	// DefaultManagerInitScriptTemplate is the default template for the init script which
	// the flavor injects into the user data of the instance to configure Docker Swarm Managers
	DefaultManagerInitScriptTemplate = `
#!/bin/sh
set -o errexit
set -o nounset
set -o xtrace

{{ if BOOTSTRAP }}
# Bootstrap
kubeadm init --token {{ KUBEADM_JOIN_TOKEN }}
export KUBECONFIG=/etc/kubernetes/admin.conf
{{ if ADDON "network" }}
    kubectl apply -f {{ ADDON "network" }}
{{ end }}
{{ if ADDON "visualise" }}
    kubectl apply -f {{ ADDON "visualise" }}
{{ end }}
{{ end }}{{/* bootstrap */}}

{{ if WORKER }}
# Manager node but because we are doing a single master control plane, we will just reuse the node as worker
kubeadm join --token {{ KUBEADM_JOIN_TOKEN }} {{ KUBE_JOIN_IP }}:{{ BIND_PORT }}
{{ end }}{{/* worker */}}

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

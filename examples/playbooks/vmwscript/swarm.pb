{{/* Runs the vmwscript playbook to provision a Swarm on VMSphere */}}
{{/* =% vmwscript %= */}}

{{ flag "vcenter-url" "string" "VCenter URL" | prompt "VCenter URL?" "string" "https://username@vsphere.local:password@vc.unifydc.io/sdk" | var "input/url"}}
{{ flag "data-center" "string" "Data Center name" | prompt "Data Center Name?" "string" "Datacenter" | var "input/dc"}}
{{ flag "data-store" "string" "Data Store name" | prompt "Data Store Name?" "string" "datastore1" | var "input/ds"}}
{{ flag "network-name" "string" "Network name" | prompt "Network Name?" "string" "Internal Network (NAT)" | var "input/nn"}}
{{ flag "vsphere-host" "string" "VSphere host" | prompt "Host Name?" "string" "exsi01.unifydc.io" | var "input/host"}}

{{ flag "user" "string" "Username" | prompt "User Name?" "string" | var "input/user"}}
{{ flag "pass" "string" "Password" | prompt "Password?" "string" | var "input/password"}}
{{ flag "stack" "string" "Stack name" | prompt "Stack?" "string" | var "input/stack"}}
{{ flag "admin-user" "string" "Admin user" | prompt "Admin User?" "string" "admin" | var "input/admin_user"}}
{{ flag "admin-pass" "string" "Admin password" | prompt "Admin Password?" "string" "adminpass" | var "input/admin_pass" }}

{{/* The flags are piped to set variables (var); now are available in the scope of this playbook template */}}
{{ include `./swarm.json` }}

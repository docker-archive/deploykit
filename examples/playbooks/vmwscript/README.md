VMWScript Playbooks
==========================

This folder contains a number of playbooks that make use of the VMware scripting engine to ease 
the deployment of Virtual Machines and Virtual Machine templates on vCenter or vSphere. 

Example Playbooks:
- Docker-EE: This creates a Docker Template and will then deploy UCP (manager) and some worker nodes
- Wordpress: Deploys a single virtual machine and installs and starts a wordpress installation

For more information about VMwscript please look at https://github.com/docker/infrakit/blob/master/pkg/x/vmwscript/readme.md
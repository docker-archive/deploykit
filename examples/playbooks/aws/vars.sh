{{/* =% sh %= */}}

{{ $project := param "project" "string" "project" | prompt "Project?" "string" "myproject" }}

{{ $cidr := param "cidr" "string" "CIDR Block" | prompt "CIDR block?" "string" "10.0.0.0/16" }}
{{ $cidrSubnet1 := param "cidrSubnet1" "string" "CIDR Block for subnet1" | prompt "CIDR block?" "string" "10.0.100.0/24" }}
{{ $cidrSubnet2 := param "cidrSubnet2" "string" "CIDR Block for subnet2" | prompt "CIDR block?" "string" "10.0.200.0/24" }}

{{ $azSubnet1 := param "azSubnet1" "string" "AZ Subnet1" | prompt "Availability Zone?" "string" "eu-central-1a" }}
{{ $azSubnet2 := param "azSubnet2" "string" "AZ Subnet2" | prompt "Availability Zone?" "string" "eu-central-1b" }}

# write to the metadata vars
{{ metadata `mystack/vars/project` $project }}
{{ metadata `mystack/vars/cidr` $cidr }}
{{ metadata `mystack/vars/subnet1/cidr` $cidrSubnet1 }}
{{ metadata `mystack/vars/subnet2/cidr` $cidrSubnet2 }}
{{ metadata `mystack/vars/subnet1/az` $azSubnet1 }}
{{ metadata `mystack/vars/subnet2/az` $azSubnet2 }}

infrakit local mystack/vars change

{{ $project := metadata `mystack/vars/project` }}

echo "Project is {{ $project }}"

infrakit local mystack/vars change

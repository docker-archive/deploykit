package main

import (
	"bytes"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/docker/infrakit/pkg/testing"
)

var terraformShowOutput = []byte(`
aws_internet_gateway.default:
  id = igw-c5fcffac
  tags.% = 1
  tags.provisioner = infrakit-terraform-demo
  vpc_id = vpc-f8d45a90
aws_route.internet_access:
  id = r-rtb-7bf68e131080289494
  destination_cidr_block = 0.0.0.0/0
  destination_prefix_list_id =
  gateway_id = igw-c5fcffac
  instance_id =
  instance_owner_id =
  nat_gateway_id =
  network_interface_id =
  origin = CreateRoute
  route_table_id = rtb-7bf68e13
  state = active
  vpc_peering_connection_id =
aws_security_group.default:
  id = sg-b903abd2
  description = Used in the terraform
  egress.# = 1
  egress.482069346.cidr_blocks.# = 1
  egress.482069346.cidr_blocks.0 = 0.0.0.0/0
  egress.482069346.from_port = 0
  egress.482069346.prefix_list_ids.# = 0
  egress.482069346.protocol = -1
  egress.482069346.security_groups.# = 0
  egress.482069346.self = false
  egress.482069346.to_port = 0
  ingress.# = 2
  ingress.2165049311.cidr_blocks.# = 1
  ingress.2165049311.cidr_blocks.0 = 10.0.0.0/16
  ingress.2165049311.from_port = 80
  ingress.2165049311.protocol = tcp
  ingress.2165049311.security_groups.# = 0
  ingress.2165049311.self = false
  ingress.2165049311.to_port = 80
  ingress.2541437006.cidr_blocks.# = 1
  ingress.2541437006.cidr_blocks.0 = 0.0.0.0/0
  ingress.2541437006.from_port = 22
  ingress.2541437006.protocol = tcp
  ingress.2541437006.security_groups.# = 0
  ingress.2541437006.self = false
  ingress.2541437006.to_port = 22
  name = terraform_example
  owner_id = 041673875206
  tags.% = 1
  tags.provisioner = infrakit-terraform-demo
  vpc_id = vpc-f8d45a90
aws_subnet.default:
  id = subnet-32feb75a
  availability_zone = eu-central-1a
  cidr_block = 10.0.1.0/24
  map_public_ip_on_launch = true
  tags.% = 1
  tags.provisioner = infrakit-terraform-demo
  vpc_id = vpc-f8d45a90
aws_vpc.default:
  id = vpc-f8d45a90
  cidr_block = 10.0.0.0/16
  default_network_acl_id = acl-9d88fef5
  default_route_table_id = rtb-7bf68e13
  default_security_group_id = sg-1403ab7f
  dhcp_options_id = dopt-b632fedf
  enable_dns_hostnames = false
  enable_dns_support = true
  instance_tenancy = default
  main_route_table_id = rtb-7bf68e13
  tags.% = 1
  tags.provisioner = infrakit-terraform-demo
`)

func TestTerraformShowParseResult(t *testing.T) {

	found, err := parseTerraformShowOutput(TResourceType("aws_vpc"), bytes.NewBuffer(terraformShowOutput))
	require.NoError(t, err)
	T(100).Info(found)
	require.Equal(t, TResourceProperties{
		"id":                        "vpc-f8d45a90",
		"cidr_block":                "10.0.0.0/16",
		"default_network_acl_id":    "acl-9d88fef5",
		"default_route_table_id":    "rtb-7bf68e13",
		"default_security_group_id": "sg-1403ab7f",
		"dhcp_options_id":           "dopt-b632fedf",
		"enable_dns_hostnames":      "false",
		"enable_dns_support":        "true",
		"instance_tenancy":          "default",
		"main_route_table_id":       "rtb-7bf68e13",
		"tags.%":                    "1",
		"tags.provisioner":          "infrakit-terraform-demo",
	}, found[TResourceName("default")])
}

func TestRunTerraformShow(t *testing.T) {

	// Run this test locally only if terraform is set up
	if SkipTests("terraform") {
		t.SkipNow()
	}

	dir, err := os.Getwd()
	require.NoError(t, err)
	dir = path.Join(dir, "aws-two-tier")

	found, err := doTerraformShow(dir, TResourceType("aws_vpc"))
	require.NoError(t, err)
	require.Equal(t, 1, len(found))
	T(100).Infoln(found)
}

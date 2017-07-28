package main

import (
	"bufio"
	"bytes"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/docker/infrakit/pkg/testing"
)

func TestTerraformShowParseResultEmpty(t *testing.T) {
	found, err := parseTerraformShowOutput(TResourceType("aws_vpc"), bytes.NewBuffer([]byte("")))
	require.NoError(t, err)
	require.Equal(t, map[TResourceName]TResourceProperties{}, found)
}

func TestTerraformShowParseResultResTypes(t *testing.T) {
	data := []byte(`
res-type1.host1:
  id = type1-host1
res-type1.host2:
  id = type1-host2
res-type2.host1:
  id = type2-host1
res-type3.host1:
  id = type3-host1`)
	found, err := parseTerraformShowOutput(TResourceType("unknown"), bytes.NewBuffer(data))
	require.NoError(t, err)
	require.Equal(t, map[TResourceName]TResourceProperties{}, found)

	found, err = parseTerraformShowOutput(TResourceType("res-type1"), bytes.NewBuffer(data))
	require.NoError(t, err)
	require.Equal(t, TResourceProperties{"id": "type1-host1"}, found[TResourceName("host1")])
	require.Equal(t, TResourceProperties{"id": "type1-host2"}, found[TResourceName("host2")])

	found, err = parseTerraformShowOutput(TResourceType("res-type2"), bytes.NewBuffer(data))
	require.NoError(t, err)
	require.Equal(t, TResourceProperties{"id": "type2-host1"}, found[TResourceName("host1")])

	found, err = parseTerraformShowOutput(TResourceType("res-type3"), bytes.NewBuffer(data))
	require.NoError(t, err)
	require.Equal(t, TResourceProperties{"id": "type3-host1"}, found[TResourceName("host1")])
}

func convertToSingleInstanceOutput(data []byte, resTypeName string) []byte {
	resType := strings.Split(resTypeName, ".")[0]
	resName := strings.Split(resTypeName, ".")[1]
	match := false
	lines := []string{}
	reader := bufio.NewReader(bytes.NewBuffer(data))
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			break
		}
		m := title.FindAllStringSubmatch(string(line), -1)
		if m != nil && len(m[0][1]) > 0 && len(m[0][2]) > 0 {
			if m[0][1] == resType && m[0][2] == resName {
				match = true
			} else if len(lines) > 0 {
				// No longer match, break if we have data
				break
			}
		} else if match {
			// Remove leading spaces
			lineStr := string(line)
			for lineStr[0:1] == " " {
				lineStr = lineStr[1:]
			}
			lines = append(lines, lineStr)
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

func TestTerraformShowParseResultEmptyValues(t *testing.T) {
	// {
	//   "id": 123,
	//   "destination_cidr_block": "0.0.0.0/0",
	//   "destination_prefix_list_id": "",
	//   "gateway_id": "",
	//   "instance_id": "",
	//   "instance_owner_id": "",
	//   "pie": 3.14
	// }
	// Code editors tend to remove the trailing whitespace above, ensure that a space exists
	// after the equals
	input := strings.Replace(`
type.host:
  id                         = 123
  destination_cidr_block     = 0.0.0.0/0
  destination_prefix_list_id =
  gateway_id                 = igw-c5fcffac
  instance_id                =
  instance_owner_id          =
  pie                        = 3.14
`, "=\n", "= \n", -1)
	found, err := parseTerraformShowOutput(TResourceType("type"), bytes.NewBuffer([]byte(input)))
	require.NoError(t, err)
	expected := TResourceProperties{
		"id": 123,
		"destination_cidr_block":     "0.0.0.0/0",
		"destination_prefix_list_id": "",
		"gateway_id":                 "igw-c5fcffac",
		"instance_id":                "",
		"instance_owner_id":          "",
		"pie":                        3.14,
	}
	require.Equal(t, expected, found[TResourceName("host")])

	// Also verify single instance output
	data := convertToSingleInstanceOutput([]byte(input), "type.host")
	props, err := parseTerraformShowForInstanceOutput(bytes.NewBuffer(data))
	require.NoError(t, err)
	require.Equal(t, expected, props)
}

func TestTerraformShowParseResultLists(t *testing.T) {
	// {
	//   "id": 1,
	//   "tags": ["tag1", "tag2"],
	//   "keys": [1, "k2", false],
	//   "z-foo": "z-bar"
	// }
	data := []byte(`
type.host:
  id = 1
  tags.# = 2
  tags.123 = tag1
  tags.234 = tag2
  keys.# = 3
  keys.123 = 1
  keys.234 = k2
  keys.345 = false
  z-foo = z-bar
`)
	found, err := parseTerraformShowOutput(TResourceType("type"), bytes.NewBuffer(data))
	require.NoError(t, err)
	expected := TResourceProperties{
		"id":    1,
		"tags":  []interface{}{"tag1", "tag2"},
		"keys":  []interface{}{1, "k2", false},
		"z-foo": "z-bar",
	}
	require.Equal(t, expected, found[TResourceName("host")])

	// Also verify single instance output
	data = convertToSingleInstanceOutput(data, "type.host")
	props, err := parseTerraformShowForInstanceOutput(bytes.NewBuffer(data))
	require.NoError(t, err)
	require.Equal(t, expected, props)
}

func TestTerraformShowParseResultMaps(t *testing.T) {
	// {
	//   "id": 1,
	//   "tags": {
	//    "tag1": "v1",
	//    "tag2": "v2"
	//   },
	//   "keys": {
	//    "key1": 1,
	//    "key2": "k2",
	//    "key3": false
	//   }
	// }
	data := []byte(`
type.host:
  id = 1
  tags.% = 2
  tags.tag1 = v1
  tags.tag2 = v2
  keys.% = 3
  keys.key1 = 1
  keys.key2 = k2
  keys.key3 = false
  z-foo = z-bar
`)
	found, err := parseTerraformShowOutput(TResourceType("type"), bytes.NewBuffer(data))
	require.NoError(t, err)
	expected := TResourceProperties{
		"id":    1,
		"tags":  map[string]interface{}{"tag1": "v1", "tag2": "v2"},
		"keys":  map[string]interface{}{"key1": 1, "key2": "k2", "key3": false},
		"z-foo": "z-bar",
	}
	require.Equal(t, expected, found[TResourceName("host")])

	// Also verify single instance output
	data = convertToSingleInstanceOutput(data, "type.host")
	props, err := parseTerraformShowForInstanceOutput(bytes.NewBuffer(data))
	require.NoError(t, err)
	require.Equal(t, expected, props)
}

func TestTerraformShowParseResultNested(t *testing.T) {
	// {
	//   "id": 1,
	//   "tags": [
	//     {"list1": [1, 2]},
	//     5,
	//     {"list2": [3, 4]},
	//     [
	//       true, false
	//     ]
	//   ]
	// }
	data := []byte(`
type.host:
  id = 1
  tags.# = 3
  tags.111.list1.# = 2
  tags.111.list1.111 = 1
  tags.111.list1.222 = 2
  tags.222 = 5
  tags.333.list2.# = 2
  tags.333.list2.111 = 3
  tags.333.list2.222 = 4
`)
	found, err := parseTerraformShowOutput(TResourceType("type"), bytes.NewBuffer(data))
	require.NoError(t, err)
	require.Equal(t, 1, len(found))
	props := found[TResourceName("host")]
	require.Equal(t, 2, len(props))
	require.Equal(t, 1, props["id"])
	// Tag list sort order not guaranteed
	tags := props["tags"].([]interface{})
	require.Equal(t, 3, len(tags))
	require.Contains(t, tags, 5)
	require.Contains(t, tags, map[string]interface{}{"list1": []interface{}{1, 2}})
	require.Contains(t, tags, map[string]interface{}{"list2": []interface{}{3, 4}})

	// Also verify single instance output
	data = convertToSingleInstanceOutput(data, "type.host")
	props, err = parseTerraformShowForInstanceOutput(bytes.NewBuffer(data))
	require.NoError(t, err)
	require.Equal(t, 2, len(props))
	require.Equal(t, 1, props["id"])
	tags = props["tags"].([]interface{})
	require.Equal(t, 3, len(tags))
	require.Contains(t, tags, 5)
	require.Contains(t, tags, map[string]interface{}{"list1": []interface{}{1, 2}})
	require.Contains(t, tags, map[string]interface{}{"list2": []interface{}{3, 4}})
}

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
ibm_compute_vm_instance.instance-1499827079:
  id = 36147555
  cores = 1
  datacenter = dal10
  file_storage_ids.# = 0
  hostname = worker-1499827079
  memory = 2048
  ssh_key_ids.# = 1
  ssh_key_ids.0 = 123456
  tags.# = 5
  tags.1516831048 = infrakit.group:workers
  tags.3434794676 = infrakit.config_sha:tubmesopo6lrsfnl5otajlpvwd23v46j
  tags.356689043 = name:instance-1499827079
  tags.3639269190 = infrakit-link-context:swarm::c80s4c4kq0kgjs64ojxzvsdjz::worker
  tags.838324444 = swarm-id:c80s4c4kq0kgjs64ojxzvsdjz
  user_metadata = set -o errexit
set -o nounset
set -o xtrace
apt-get -y update
FOO=BAR
echo $FOO
  z_prop = z_val
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

func TestTerraformShowParseResultTagsList(t *testing.T) {

	found, err := parseTerraformShowOutput(TResourceType("ibm_compute_vm_instance"), bytes.NewBuffer(terraformShowOutput))
	require.NoError(t, err)
	expected := TResourceProperties{
		"id":               36147555,
		"cores":            1,
		"datacenter":       "dal10",
		"file_storage_ids": []interface{}{},
		"hostname":         "worker-1499827079",
		"memory":           2048,
		"ssh_key_ids":      []interface{}{123456},
		"tags": []interface{}{
			"infrakit.group:workers",
			"infrakit.config_sha:tubmesopo6lrsfnl5otajlpvwd23v46j",
			"name:instance-1499827079",
			"infrakit-link-context:swarm::c80s4c4kq0kgjs64ojxzvsdjz::worker",
			"swarm-id:c80s4c4kq0kgjs64ojxzvsdjz",
		},
		"user_metadata": "set -o errexit\nset -o nounset\nset -o xtrace\napt-get -y update\nFOO=BAR\necho $FOO",
		"z_prop":        "z_val",
	}
	require.Equal(t, expected, found[TResourceName("instance-1499827079")])

	// Also verify single instance output
	data := convertToSingleInstanceOutput(terraformShowOutput, "ibm_compute_vm_instance.instance-1499827079")
	props, err := parseTerraformShowForInstanceOutput(bytes.NewBuffer(data))
	require.NoError(t, err)
	require.Equal(t, expected, props)
}

func TestTerraformShowParseResultAwsVpc(t *testing.T) {
	found, err := parseTerraformShowOutput(TResourceType("aws_vpc"), bytes.NewBuffer(terraformShowOutput))
	require.NoError(t, err)
	expected := TResourceProperties{
		"id":                        "vpc-f8d45a90",
		"cidr_block":                "10.0.0.0/16",
		"default_network_acl_id":    "acl-9d88fef5",
		"default_route_table_id":    "rtb-7bf68e13",
		"default_security_group_id": "sg-1403ab7f",
		"dhcp_options_id":           "dopt-b632fedf",
		"enable_dns_hostnames":      false,
		"enable_dns_support":        true,
		"instance_tenancy":          "default",
		"main_route_table_id":       "rtb-7bf68e13",
		"tags":                      map[string]interface{}{"provisioner": "infrakit-terraform-demo"},
	}
	require.Equal(t, expected, found[TResourceName("default")])

	// Also verify single instance output
	data := convertToSingleInstanceOutput(terraformShowOutput, "aws_vpc.default")
	props, err := parseTerraformShowForInstanceOutput(bytes.NewBuffer(data))
	require.NoError(t, err)
	require.Equal(t, expected, props)
}

func TestTerraformShowParseResultAwsSubnet(t *testing.T) {
	found, err := parseTerraformShowOutput(TResourceType("aws_subnet"), bytes.NewBuffer(terraformShowOutput))
	require.NoError(t, err)
	expected := TResourceProperties{
		"id":                      "subnet-32feb75a",
		"availability_zone":       "eu-central-1a",
		"cidr_block":              "10.0.1.0/24",
		"map_public_ip_on_launch": true,
		"tags":   map[string]interface{}{"provisioner": "infrakit-terraform-demo"},
		"vpc_id": "vpc-f8d45a90",
	}
	require.Equal(t, expected, found[TResourceName("default")])

	// Also verify single instance output
	data := convertToSingleInstanceOutput(terraformShowOutput, "aws_subnet.default")
	props, err := parseTerraformShowForInstanceOutput(bytes.NewBuffer(data))
	require.NoError(t, err)
	require.Equal(t, expected, props)
}

func TestTerraformShowParseResultAwsSecurityGroup(t *testing.T) {
	found, err := parseTerraformShowOutput(TResourceType("aws_security_group"), bytes.NewBuffer(terraformShowOutput))
	require.NoError(t, err)
	require.Equal(t, 1, len(found))
	props := found[TResourceName("default")]
	// List sort order for "ingress" is not guananteed, check separately
	ingress := props["ingress"].([]interface{})
	delete(props, "ingress")
	require.Equal(t, 2, len(ingress))
	expectedIngress1 := map[string]interface{}{
		"cidr_blocks":     []interface{}{"10.0.0.0/16"},
		"from_port":       80,
		"protocol":        "tcp",
		"security_groups": []interface{}{},
		"self":            false,
		"to_port":         80,
	}
	expectedIngress2 := map[string]interface{}{
		"cidr_blocks":     []interface{}{"0.0.0.0/0"},
		"from_port":       22,
		"protocol":        "tcp",
		"security_groups": []interface{}{},
		"self":            false,
		"to_port":         22,
	}
	require.Contains(t, ingress, expectedIngress1)
	require.Contains(t, ingress, expectedIngress2)
	// Verify everything else
	expected := TResourceProperties{
		"id":          "sg-b903abd2",
		"description": "Used in the terraform",
		"egress": []interface{}{
			map[string]interface{}{
				"cidr_blocks":     []interface{}{"0.0.0.0/0"},
				"from_port":       0,
				"prefix_list_ids": []interface{}{},
				"protocol":        -1,
				"security_groups": []interface{}{},
				"self":            false,
				"to_port":         0,
			},
		},
		"name":     "terraform_example",
		"owner_id": 41673875206,
		"tags": map[string]interface{}{
			"provisioner": "infrakit-terraform-demo",
		},
		"vpc_id": "vpc-f8d45a90",
	}
	require.Equal(t, expected, props)

	// Also verify single instance output
	data := convertToSingleInstanceOutput(terraformShowOutput, "aws_security_group.default")
	props, err = parseTerraformShowForInstanceOutput(bytes.NewBuffer(data))
	require.NoError(t, err)
	ingress = props["ingress"].([]interface{})
	delete(props, "ingress")
	require.Equal(t, 2, len(ingress))
	require.Contains(t, ingress, expectedIngress1)
	require.Contains(t, ingress, expectedIngress2)
	require.Equal(t, expected, props)
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

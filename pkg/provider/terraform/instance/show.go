package main

import (
	"bufio"
	"io"
	"regexp"

	"github.com/docker/infrakit/pkg/util/exec"
)

var title = regexp.MustCompile("^(.*)\\.(.*):$")
var properties = regexp.MustCompile("[ ]+(.*)[ ]+=[ ]+(.*)$")

// TerraformShow calls terraform show and scans the result for the given resource type
// and returns a map of properties by name.
//
// Terraform has done little to output data in JSON format (no options in terraform show in v0.9).
// Thus they are preventing interoperability and enforces lock-in. The terraform-api effort has been
// shutdown as well.  Users who have large data sets are thus locked in since terraform show itself
// isn't very scalable when dealing with large number of resources under management.
//
// Example of terraform show
// aws_subnet.default:
//   id = subnet-32feb75a
//   availability_zone = eu-central-1a
//   cidr_block = 10.0.1.0/24
//   map_public_ip_on_launch = true
//   tags.% = 1
//   tags.provisioner = infrakit-terraform-demo
//   vpc_id = vpc-f8d45a90
// aws_vpc.default:
//   id = vpc-f8d45a90
//   cidr_block = 10.0.0.0/16
//   default_network_acl_id = acl-9d88fef5
//   default_route_table_id = rtb-7bf68e13
//   default_security_group_id = sg-1403ab7f
//   dhcp_options_id = dopt-b632fedf
//   enable_dns_hostnames = false
//   enable_dns_support = true
//   instance_tenancy = default
//   main_route_table_id = rtb-7bf68e13
//   tags.% = 1
//   tags.provisioner = infrakit-terraform-demo
func parseTerraformShowOutput(byType TResourceType, input io.Reader) (map[TResourceName]TResourceProperties, error) {
	found := map[TResourceName]TResourceProperties{}

	reader := bufio.NewReader(input)
	var resourceName TResourceName
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			break
		}

		m := title.FindAllStringSubmatch(string(line), -1)
		if m != nil && len(m[0][1]) > 0 && len(m[0][1]) > 0 {

			resourceType := TResourceType(m[0][1])
			resourceName = TResourceName(m[0][2])

			if resourceType != byType {
				continue
			}
			found[resourceName] = TResourceProperties{}

		} else {
			p := properties.FindAllStringSubmatch(string(line), -1)
			if p != nil && len(p[0][1]) > 0 && len(p[0][2]) > 0 {
				key := p[0][1]
				value := p[0][2]

				if props, has := found[resourceName]; has {
					props[key] = value
				}
			}
		}
	}
	return found, nil
}

// doTerraformShow shells out to run `terraform show` and parses the result
func doTerraformShow(dir string,
	resourceType TResourceType) (result map[TResourceName]TResourceProperties, err error) {

	command := exec.Command(`terraform show`).InheritEnvs(true).WithDir(dir)
	command.StartWithHandlers(
		nil,
		func(r io.Reader) error {
			found, err := parseTerraformShowOutput(resourceType, r)
			result = found
			return err
		},
		nil)

	command.Wait()
	return
}

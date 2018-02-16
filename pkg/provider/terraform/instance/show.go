package instance

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// title matches a line like: "aws_security_group.default:"
var title = regexp.MustCompile("^(.*)\\.(.*):$")

// properties matches a line like: "  id = igw-c5fcffac"
var properties = regexp.MustCompile("^[ ]+(.*)[ ]+=[ ]+(.*)$")

// propertiesForInstance matches a line like: "id = igw-c5fcffac"
var propertiesForInstance = regexp.MustCompile("^(.*)[ ]+=[ ]+(.*)$")

// listRegx matches a kye in a line like: "  egress.# = 1"
var sliceRegex = regexp.MustCompile("^([^.]+)\\.#")

// mapInsliceRegex matches a key in a line like: "  egress.482069346.cidr_blocks.# = 1"
var mapInSliceRegex = regexp.MustCompile("^([^.]+)\\.([0-9]+)\\.(.*)")

// mapRegex matches a key in a line like: "  tags.% = 1"
var mapRegex = regexp.MustCompile("^([^.]+)\\.%")

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
// ibm_compute_vm_instance.instance-1499827079:
//   id = 36147555
//   cores = 1
//   datacenter = dal10
//   file_storage_ids.# = 0
//   hostname = worker-1499827079
//   memory = 2048
//   ssh_key_ids.# = 1
//   ssh_key_ids.0 = 123456
//   tags.# = 5
//   tags.1516831048 = infrakit.group:workers
//   tags.3434794676 = infrakit.config.hash:tubmesopo6lrsfnl5otajlpvwd23v46j
//   tags.356689043 = name:instance-1499827079
//   tags.3639269190 = infrakit-link-context:swarm::c80s4c4kq0kgjs64ojxzvsdjz::worker
//   tags.838324444 = infrakit.cluster.id:c80s4c4kq0kgjs64ojxzvsdjz
//   user_metadata = set -o errexit
// set -o nounset
// set -o xtrace
// apt-get -y update
func parseTerraformShowOutput(resTypes []TResourceType, propFilter []string, input io.Reader) (map[TResourceType]map[TResourceName]TResourceProperties, error) {
	// Convert types to map for filtering on resource types
	resTypeFilter := make(map[TResourceType]struct{}, len(resTypes))
	for _, resType := range resTypes {
		resTypeFilter[resType] = struct{}{}
	}
	// Track property key for multi-line property
	var propKey string
	// Track properties for current resource
	var props TResourceProperties

	results := map[TResourceType]map[TResourceName]TResourceProperties{}
	reader := bufio.NewReader(input)
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			break
		}

		m := title.FindAllStringSubmatch(string(line), -1)
		if m != nil && len(m[0][1]) > 0 && len(m[0][2]) > 0 {
			// Line is for a new resource, verify supported type
			resType := TResourceType(m[0][1])
			if _, has := resTypeFilter[resType]; !has {
				props = nil
				continue
			}
			resourceName := TResourceName(m[0][2])
			var resNamePropMap map[TResourceName]TResourceProperties
			if entry, has := results[resType]; has {
				resNamePropMap = entry
			} else {
				resNamePropMap = make(map[TResourceName]TResourceProperties)
				results[resType] = resNamePropMap
			}
			props = TResourceProperties{}
			resNamePropMap[resourceName] = props
		} else if props != nil {
			p := properties.FindAllStringSubmatch(string(line), -1)
			if p != nil && len(p[0][1]) > 0 {
				propKey = strings.TrimSpace(p[0][1])
				value := strings.TrimSpace(p[0][2])
				props[propKey] = value
			} else {
				// Append to previous key
				props[propKey] = fmt.Sprintf("%s\n%s", props[propKey], line)
			}
		}
	}

	// Process the properties to convert from string to native types
	for _, resNameProps := range results {
		for _, props := range resNameProps {
			expandProps(props)
			// TODO(kaufers): Move this filtering to where the lines are being processed
			if propFilter != nil && len(propFilter) > 0 {
				propMap := make(map[string]struct{}, len(propFilter))
				for _, propID := range propFilter {
					propMap[propID] = struct{}{}
				}
				for propID := range props {
					if _, has := propMap[propID]; !has {
						delete(props, propID)
					}
				}
			}
		}
	}
	return results, nil
}

// parseTerraformShowForInstanceOutput calls terraform show for a specific resource
//
// Example of terraform state show <resource>
// id = subnet-32feb75a
// availability_zone = eu-central-1a
// cidr_block = 10.0.1.0/24
// map_public_ip_on_launch = true
// tags.% = 1
// tags.provisioner = infrakit-terraform-demo
// vpc_id = vpc-f8d45a90
func parseTerraformShowForInstanceOutput(input io.Reader) (TResourceProperties, error) {
	reader := bufio.NewReader(input)
	var propKey string
	props := TResourceProperties{}
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			break
		}
		p := propertiesForInstance.FindAllStringSubmatch(string(line), -1)
		if p != nil && len(p[0][1]) > 0 {
			propKey = strings.TrimSpace(p[0][1])
			value := strings.TrimSpace(p[0][2])
			props[propKey] = value
		} else {
			// Append to previous key
			props[propKey] = fmt.Sprintf("%s\n%s", props[propKey], line)
		}
	}
	// Process the properties to convert from string to native types
	expandProps(props)
	return props, nil
}

// expandProps converts the flattened resource definition into native JSON types
func expandProps(props map[string]interface{}) {
	// Sort all keys to group maps and slices
	keys := make([]string, len(props))
	i := 0
	for key := range props {
		keys[i] = key
		i++
	}
	sort.Strings(keys)

	for i, key := range keys {
		// Map keys in form: <key>.%
		if m := mapRegex.FindAllStringSubmatch(key, -1); len(m) > 0 {
			prefix := m[0][1]
			nestedProps := map[string]interface{}{}
			// Get all of the keys with the same prefix
			for (i + 1) < len(keys) {
				if !strings.HasPrefix(keys[i+1], prefix+".") {
					break
				}
				i++
				curKey := keys[i]
				// Remove common prefix and add to nested map
				nestedKey := strings.Replace(curKey, prefix+".", "", 1)
				nestedProps[nestedKey] = props[curKey]
			}
			// No more additional keys, process the properties in the map and associate
			// with the common key
			expandProps(nestedProps)
			props[prefix] = nestedProps
			continue
		}
		// List keys in form: <key>.#
		if m := sliceRegex.FindAllStringSubmatch(key, -1); len(m) > 0 {
			prefix := m[0][1]
			slice := []interface{}{}
			nestedProps := map[string]interface{}{}
			// Get all of the keys with the same prefix
			for (i + 1) < len(keys) {
				if !strings.HasPrefix(keys[i+1], prefix+".") {
					break
				}
				i++
				curKey := keys[i]
				// Map in slice in form: <prefix>.482069346.cidr_blocks.#
				if m := mapInSliceRegex.FindAllStringSubmatch(curKey, -1); len(m) > 0 {
					nestedPropsKey := m[0][2] // Common hash for map values, used to group
					suffix := m[0][3]
					if nestedMap, has := nestedProps[nestedPropsKey]; has {
						nestedMap.(map[string]interface{})[suffix] = props[curKey]
					} else {
						nestedProps[nestedPropsKey] = map[string]interface{}{
							suffix: props[curKey],
						}
					}
				} else {
					slice = append(slice, convertToType(props[curKey].(string)))
				}
			}
			for _, nestedMap := range nestedProps {
				expandProps(nestedMap.(map[string]interface{}))
				slice = append(slice, nestedMap)
			}
			props[prefix] = slice
			continue
		}
	}
	for _, key := range keys {
		// Ignore list and map key values, they have already been expanded
		if strings.Contains(key, ".") {
			delete(props, key)
			continue
		}
		val := props[key]
		props[key] = convertToType(val.(string))
	}
}

// convertToType converts the string to a native int, float, or bool type
func convertToType(val string) interface{} {
	if intVal, err := strconv.Atoi(val); err == nil {
		return intVal
	}
	if floatVal, err := strconv.ParseFloat(val, 64); err == nil {
		return floatVal
	}
	if boolVar, err := strconv.ParseBool(val); err == nil {
		return boolVar
	}
	return val
}

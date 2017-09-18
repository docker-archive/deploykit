package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	plugin_base "github.com/docker/infrakit/pkg/plugin"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	terraform "github.com/docker/infrakit/pkg/provider/terraform/instance"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

func mustHaveTerraform() {
	// check if terraform exists
	if _, err := exec.LookPath("terraform"); err != nil {
		log.Error("Cannot find terraform.  Please install at https://www.terraform.io/downloads.html")
		os.Exit(1)
	}
}

func getDir() string {
	dir := os.Getenv("INFRAKIT_INSTANCE_TERRAFORM_DIR")
	if dir != "" {
		return dir
	}
	return os.TempDir()
}

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Terraform instance plugin",
	}
	name := cmd.Flags().String("name", "instance-terraform", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	dir := cmd.Flags().String("dir", getDir(), "Dir for storing plan files")
	pollInterval := cmd.Flags().Duration("poll-interval", 30*time.Second, "Terraform polling interval")
	standalone := cmd.Flags().Bool("standalone", false, "Set if running standalone, disables manager leadership verification")
	// Import options
	importGrpSpecURL := cmd.Flags().String("import-group-spec-url", "", "Defines the group spec that the instance is imported into")
	importResources := cmd.Flags().StringArray("import-resource", []string{}, "Defines the resource to import in the format <type>:[<name>:]<id>")
	importGrpID := cmd.Flags().String("import-group-id", "", "Defines the group ID to import the resource into (optional)")

	cmd.Run = func(c *cobra.Command, args []string) {
		mustHaveTerraform()
		importInstSpec, err := parseInstanceSpecFromGroup(*importGrpSpecURL, *importGrpID)
		if err != nil {
			// If we cannot parse the group spec then we cannot import the resource, the plugin should
			// not start since terraform is not managing the resource
			log.Error(err)
			panic(err)
		}
		resources := []*terraform.ImportResource{}
		for _, resourceString := range *importResources {
			split := strings.Split(resourceString, ":")
			if len(split) < 2 || len(split) > 3 {
				err := fmt.Errorf("Imported resource value is not valid: %v", resourceString)
				log.Error(err)
				panic(err)
			}
			resType := terraform.TResourceType(split[0])
			var resName string
			var resID string
			if len(split) == 3 {
				resName = split[1]
				resID = split[2]
			} else {
				resID = split[1]
			}
			res := terraform.ImportResource{
				ResourceID:   &resID,
				ResourceType: &resType,
			}
			if resName != "" {
				tResName := terraform.TResourceName(resName)
				res.ResourceName = &tResName
			}
			resources = append(resources, &res)
		}
		importOpts := terraform.ImportOptions{
			InstanceSpec: importInstSpec,
			Resources:    resources,
		}
		cli.SetLogLevel(*logLevel)
		run.Plugin(plugin_base.DefaultTransport(*name), instance_plugin.PluginServer(
			terraform.NewTerraformInstancePlugin(*dir, *pollInterval, *standalone, &importOpts)),
		)
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

// parseInstanceSpecFromGroup parses the instance.Spec from the group.Spec and adds
// in the tags that should be set on the imported instance
func parseInstanceSpecFromGroup(groupSpecURL, groupID string) (*instance.Spec, error) {
	// TODO: Support a URL to a manager config with multiple nested groups
	if groupSpecURL == "" {
		log.Infof("No group spec URL specified for import")
		return nil, nil
	}
	var groupSpec group.Spec
	t, err := template.NewTemplate(groupSpecURL, template.Options{MultiPass: false})
	if err != nil {
		return nil, err
	}
	template, err := t.Render(nil)
	if err != nil {
		return nil, err
	}
	if err = types.AnyString(template).Decode(&groupSpec); err != nil {
		return nil, err
	}
	// Get the instance properties we care about
	groupProps, err := group_types.ParseProperties(groupSpec)
	if err != nil {
		return nil, err
	}

	// Add in the bootstrap tag and (if set) the group ID
	tags := map[string]string{
		"infrakit.config_sha": "bootstrap",
	}
	// The group ID should match the spec
	if groupID != "" {
		if string(groupSpec.ID) != groupID {
			return nil, fmt.Errorf("Given spec ID '%v' does not match given group ID '%v'",
				string(groupSpec.ID), groupID)
		}
		tags["infrakit.group"] = groupID
	}
	// Use the first logical ID if set
	if len(groupProps.Allocation.LogicalIDs) > 0 {
		tags["LogicalID"] = string(groupProps.Allocation.LogicalIDs[0])
	}

	spec := instance.Spec{
		Properties: groupProps.Instance.Properties,
		Tags:       tags,
	}
	log.Infof("Successfully processed instance spec from group '%v': %v",
		string(groupSpec.ID),
		spec,
	)
	return &spec, nil
}

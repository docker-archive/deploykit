package vmwscript

import (
	"fmt"

	logutil "github.com/docker/infrakit/pkg/log"
)

var cmdResults = map[string]string{}

var log = logutil.New("module", "x/vmwscript")
var debugV = logutil.V(200) // 100-500 are for typical debug levels, > 500 for highly repetitive logs (e.g. from polling)

// DeploymentPlan is the top-level structure that holds the user's specification / plan
// as well as runtime information as it executes.
type DeploymentPlan struct {
	Label      string           `json:"label"`
	Version    string           `json:"version"`
	Deployment []DeploymentTask `json:"deployment"`
	VMWConfig  VMConfig         `json:"vmconfig,omitempty"`

	deploymentCounter int //defaults to 0
	commandCounter    int //defaults to 0
}

// VMConfig - This struct is used to hold all of the configuration settings that will be required to communicate with VMware
type VMConfig struct {
	VCenterURL     string `json:"vcenterURL,omitempty"`
	DCName         string `json:"datacentre,omitempty"`
	DSName         string `json:"datastore,omitempty"`
	NetworkName    string `json:"network,omitempty"`
	VSphereHost    string `json:"host,omitempty"`
	Template       string `json:"template,omitempty"`
	VMTemplateAuth struct {
		Username string `json:"guestUser,omitempty"`
		Password string `json:"guestPass,omitempty"`
	} `json:"guestCredentials,omitempty"`

	// These are used for testing a command against a pre-existing virtual machine
	Command string `json:"command,omitempty"`
	VMName  string `json:"vmname,omitempty"`
}

// DeploymentTask - is passed to the vSphere API functions in order to be executed on a remote VM
type DeploymentTask struct {
	Name string `json:"name"`
	Note string `json:"note"`
	Task struct {
		InputTemplate string              `json:"inputTemplate"`
		OutputName    string              `json:"outputName"`
		OutputType    string              `json:"outputType"`
		Network       *NetworkConfig      `json:"networkConfig,omitempty"`
		Version       string              `json:"version"`
		Commands      []DeploymentCommand `json:"commands"`
	} `json:"task"`
}

// NetworkConfig - provides distro specific networking configuration
type NetworkConfig struct {
	Distro string `json:"distro"` // this is required so that we can apply logic to the config based upon the Linux distro used.

	DeviceName string `json:"device"`

	Address string `json:"address"`
	Gateway string `json:"gateway"`
	DNS     string `json:"dns,omitempty"`

	Hostname string `json:"hostname"`

	SudoUser string `json:"sudoUser,omitempty"`
}

// DeploymentCommand - is passed to the vSphere API functions in order to be executed on a remote VM
type DeploymentCommand struct {
	CMDType string `json:"type"` //defines the type of command
	CMDNote string `json:"note"` //defines a notice that the end user will recieve

	CMD          string `json:"cmd"`              //path to either an executable or file to download
	CMDUser      string `json:"sudoUser"`         //Use sudo to execute the command
	CMDkey       string `json:"execKey"`          //Execute a line stored in the map
	CMDresultKey string `json:"resultKey"`        //Will add the contents of the file downloaded to a map under this key
	CMDIgnore    bool   `json:"ignore,omitempty"` //ignore the outcome of the task

	CMDFilePath string `json:"filePath"`         //Path to a file to download
	CMDDelete   bool   `json:"delAfterDownload"` //remove the file once downloaded
}

// //OpenFile opens a file, check file can be read and also checks the format and returns a parsed plan
// func OpenFile(filePath string) (*DeploymentPlan, error) {

// 	// Attempt to open file
// 	deploymentFile, err := os.Open(filePath)
// 	defer deploymentFile.Close()
// 	if err != nil {
// 		return nil, err
// 	}
// 	// Attempt to parse JSON
// 	jsonParser := json.NewDecoder(deploymentFile)

// 	plan := DeploymentPlan{}
// 	err = jsonParser.Decode(&plan)
// 	if err != nil {
// 		return nil, fmt.Errorf("Error Parsing JSON: %v", err)
// 	}

// 	log.Info(fmt.Sprintf("Finished parsing [%s], [%d] tasks will be deployed", plan.Label, len(plan.Deployment)))
// 	return &plan, nil
// }

// The following functions all provide the functionality to traverse through the list of commands
// and provide a stable way of passing the commands to the VMware tools

// NextDeployment returns the Command Path, the Arguments or an error
func (plan *DeploymentPlan) NextDeployment() *DeploymentTask {
	if plan.deploymentCounter > len(plan.Deployment) {
		return nil
	}

	defer func() { plan.deploymentCounter++ }()
	return &plan.Deployment[plan.deploymentCounter]
}

// DeploymentCount returns the number of commands to be executed for use in a loop
func (plan DeploymentPlan) DeploymentCount() int {
	return len(plan.Deployment)
}

//NextCommand returns the Command Path, the Arguments or an error
func (plan *DeploymentPlan) NextCommand(deployment *DeploymentTask) *DeploymentCommand {

	if plan.commandCounter > len(deployment.Task.Commands) {
		plan.commandCounter = 0 // reset counter for next set of commands
		return nil
	}

	defer func() { plan.commandCounter++ }()
	return &deployment.Task.Commands[plan.commandCounter]
}

// ResetCounter resets the counter back to zero
func (plan *DeploymentPlan) ResetCounter() {
	plan.commandCounter = 0
}

// CommandCount returns the number of commands to be executed for use in a loop
func CommandCount(deployment *DeploymentTask) int {
	return len(deployment.Task.Commands)
}

func checkRequired(v *string, message string, args ...interface{}) error {
	if v == nil || *v == "" {
		return fmt.Errorf(message, args...)
	}
	return nil
}

// Validate checks the setup and reports any errors
func (plan *DeploymentPlan) Validate() error {

	err := checkRequired(&plan.VMWConfig.VCenterURL,
		"VMware vCenter/vSphere credentials are missing")
	if err != nil {
		return err
	}

	err = checkRequired(&plan.VMWConfig.DCName,
		"No Datacenter was specified, will try to use the default (will cause errors with Linked-Mode)")
	if err != nil {
		return err
	}

	err = checkRequired(&plan.VMWConfig.DSName,
		"A VMware vCenter datastore is required for provisioning")
	if err != nil {
		return err
	}

	err = checkRequired(&plan.VMWConfig.NetworkName,
		"Specify a Network to connect to")
	if err != nil {
		return err
	}

	err = checkRequired(&plan.VMWConfig.VSphereHost,
		"A Host inside of vCenter/vSphere is required to provision on for VM capacity")
	if err != nil {
		return err
	}

	// Ideally these should be populated as they're needed for a lot of the tasks.
	err = checkRequired(&plan.VMWConfig.VMTemplateAuth.Username,
		"No Username for inside of the Guest OS was specified, somethings may fail")
	if err != nil {
		return err
	}

	err = checkRequired(&plan.VMWConfig.VMTemplateAuth.Password,
		"No Password for inside of the Guest OS was specified, somethings may fail")
	if err != nil {
		return err
	}

	if plan.VMWConfig.VCenterURL == "" || plan.VMWConfig.DSName == "" || plan.VMWConfig.VSphereHost == "" {
		return fmt.Errorf("Missing VSphere host")
	}

	return nil
}

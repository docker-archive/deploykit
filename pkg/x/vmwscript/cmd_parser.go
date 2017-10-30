package vmwscript

import (
	"encoding/json"
	"fmt"
	"os"

	logutil "github.com/docker/infrakit/pkg/log"
)

var cmdResults = map[string]string{}

var log = logutil.New("module", "x/vmwscript")
var debugV = logutil.V(200) // 100-500 are for typical debug levels, > 500 for highly repetitive logs (e.g. from polling)

type deploymentPlan struct {
	Label      string           `json:"label"`
	Version    string           `json:"version"`
	Deployment []DeploymentTask `json:"deployment"`
	VMWConfig  VMConfig         `json:"vmconfig,omitempty"`
}

// VMConfig - This struct is used to hold all of the configuration settings that will be required to communicate with VMware
type VMConfig struct {
	VCenterURL     *string `json:"vcenterURL,omitempty"`
	DCName         *string `json:"datacentre,omitempty"`
	DSName         *string `json:"datastore,omitempty"`
	NetworkName    *string `json:"network,omitempty"`
	VSphereHost    *string `json:"host,omitempty"`
	Template       *string `json:"template,omitempty"`
	VMTemplateAuth struct {
		Username *string `json:"guestUser,omitempty"`
		Password *string `json:"guestPass,omitempty"`
	} `json:"guestCredentials,omitempty"`
}

// DeploymentTask - is passed to the vSphere API functions in order to be executed on a remote VM
type DeploymentTask struct {
	Name string `json:"name"`
	Note string `json:"note"`
	Task struct {
		InputTemplate string `json:"inputTemplate"`
		OutputName    string `json:"outputName"`
		OutputType    string `json:"outputType"`

		Version  string              `json:"version"`
		Commands []DeploymentCommand `json:"commands"`
	} `json:"task"`
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

var plan *deploymentPlan
var deploymentCounter int //defaults to 0
var commandCounter int    //defaults to 0

// InitDeployment - Allocates barebones deployment plan
func InitDeployment() {
	plan = new(deploymentPlan)
}

//OpenFile - This will open a file, check file can be read and also checks the format
func OpenFile(filePath string) error {

	// Attempt to open file
	deploymentFile, err := os.Open(filePath)
	defer deploymentFile.Close()
	if err != nil {
		return err
	}
	// Attempt to parse JSON
	jsonParser := json.NewDecoder(deploymentFile)
	if plan == nil {
		log.Info("Code isn't initialising the Deployment Plan, intitialising automatically")
		InitDeployment()
	}
	err = jsonParser.Decode(&plan)
	if err != nil {
		return fmt.Errorf("Error Parsing JSON: %v", err)
	}

	log.Info("Finished parsing [%s], [%d] tasks will be deployed", plan.Label, len(plan.Deployment))
	return nil
}

// The following functions all provide the functionality to traverse through the list of commands
// and provide a stable way of passing the commands to the VMware tools

//NextDeployment - This will return the Command Path, the Arguments or an error
func NextDeployment() *DeploymentTask {
	if deploymentCounter > len(plan.Deployment) {
		return nil
	}

	defer func() { deploymentCounter++ }()
	return &plan.Deployment[deploymentCounter]
}

// DeploymentCount - Returns the number of commands to be executed for use in a loop
func DeploymentCount() int {
	return len(plan.Deployment)
}

//NextCommand - This will return the Command Path, the Arguments or an error
func NextCommand(deployment *DeploymentTask) *DeploymentCommand {

	if commandCounter > len(deployment.Task.Commands) {
		commandCounter = 0 // reset counter for next set of commands
		return nil
	}

	defer func() { commandCounter++ }()
	return &deployment.Task.Commands[commandCounter]
}

// ResetCounter - resets the counter back to zero
func ResetCounter() {
	commandCounter = 0
}

// CommandCount - Returns the number of commands to be executed for use in a loop
func CommandCount(deployment *DeploymentTask) int {
	return len(deployment.Task.Commands)
}

//VMwareConfig -
func VMwareConfig() *VMConfig {
	return &plan.VMWConfig
}

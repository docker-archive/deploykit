package aws

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/provisioners/api"
	"golang.org/x/net/context"
	"time"
)

func dummy(m string) func(ctx context.Context, cred api.Credential, req api.MachineRequest) <-chan interface{} {
	return func(ctx context.Context, cred api.Credential, req api.MachineRequest) <-chan interface{} {
		out := make(chan interface{})
		go func() {
			time.Sleep(5 * time.Second)
			log.Infoln(m)
			close(out)
		}()
		return out
	}
}

var (
	SSHKeyGen = api.Task{
		Name:    api.TaskName("ssh-key-gen"),
		Message: "Generating ssh key for host",
		Do:      dummy("SSHKeyGen"),
	}

	CreateInstance = api.Task{
		Name:    api.TaskName("create-instance"),
		Message: "Creating instance",
		Do:      dummy("CreateInstance"),
	}

	UserData = api.Task{
		Name:    api.TaskName("user-data"),
		Message: "Copy user data",
		Do:      dummy("UserData"),
	}

	InstallEngine = api.Task{
		Name:    api.TaskName("install-engine"),
		Message: "Install docker engine",
		Do:      dummy("InstallEngine"),
	}
)

func init() {
	libmachete.RegisterContextBuilder(ProvisionerName, BuildContextFromKVPair)
	libmachete.RegisterCredentialer(ProvisionerName, NewCredential)
	libmachete.RegisterTemplateBuilder(ProvisionerName, NewMachineRequest)
	libmachete.RegisterMachineRequestBuilder(ProvisionerName, NewMachineRequest)

	libmachete.RegisterTask(ProvisionerName, SSHKeyGen)
	libmachete.RegisterTask(ProvisionerName, CreateInstance)
	libmachete.RegisterTask(ProvisionerName, UserData)
	libmachete.RegisterTask(ProvisionerName, InstallEngine)
}

const (
	// ProvisionerName is a unique name for this provisioner.
	// It is used in all API / CLI to identify the provisioner.
	ProvisionerName = "aws"

	// ProvisionerVersion is a version string that is used by the provisioner to track
	// version of persisted machine requests
	ProvisionerVersion = "0.1"
)

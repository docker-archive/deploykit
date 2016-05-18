package aws

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/provisioners/api"
	"golang.org/x/net/context"
	"time"
)

func dummy(m string) api.TaskHandler {
	return func(ctx context.Context, cred api.Credential, req api.MachineRequest, events chan<- interface{}) error {
		time.Sleep(5 * time.Second)
		log.Infoln(m)
		return nil
	}
}

func init() {
	libmachete.RegisterContextBuilder(ProvisionerName, BuildContextFromKVPair)
	libmachete.RegisterCredentialer(ProvisionerName, NewCredential)
	libmachete.RegisterTemplateBuilder(ProvisionerName, NewMachineRequest)
	libmachete.RegisterMachineRequestBuilder(ProvisionerName, NewMachineRequest)
	libmachete.RegisterProvisionerBuilder(ProvisionerName, ProvisionerWith)
}

const (
	// ProvisionerName is a unique name for this provisioner.
	// It is used in all API / CLI to identify the provisioner.
	ProvisionerName = "aws"

	// ProvisionerVersion is a version string that is used by the provisioner to track
	// version of persisted machine requests
	ProvisionerVersion = "0.1"
)

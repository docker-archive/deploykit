package azure

import (
	"fmt"
	"github.com/docker/libmachete/provisioners/spi"
	"reflect"
)

// CreateInstanceRequest is the struct used to create new instances.
type CreateInstanceRequest struct {
	spi.BaseMachineRequest `yaml:",inline"`
	NetworkName            string `yaml:"network_name" json:"network_name"`
}

// Validate checks the data and returns error if not valid
func (req CreateInstanceRequest) Validate() error {
	// TODO finish this.
	return nil
}

func checkCredential(cred spi.Credential) (c *credential, err error) {
	is := false
	if c, is = cred.(*credential); !is {
		err = fmt.Errorf("credential type mismatch: %v", reflect.TypeOf(cred))
		return
	}
	return
}

func checkMachineRequest(req spi.MachineRequest) (r *CreateInstanceRequest, err error) {
	is := false
	if r, is = req.(*CreateInstanceRequest); !is {
		err = fmt.Errorf("request type mismatch: %v", reflect.TypeOf(req))
		return
	}
	return
}

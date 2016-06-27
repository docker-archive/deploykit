package azure

import (
	"github.com/docker/libmachete/provisioners/spi"
)

type createInstanceRequest struct {
	spi.BaseMachineRequest `yaml:",inline"`
	NetworkName            string `yaml:"network_name" json:"network_name"`
}

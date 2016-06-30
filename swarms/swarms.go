package swarms

import (
	"errors"
	"github.com/docker/libmachete/api"
	"github.com/docker/libmachete/machines"
	"io"
)

type swarms struct {
	provisioners machines.MachineProvisioners
	templates    api.Templates
	credentials  api.Credentials
}

// New creates a swarm manager instance.
func New(
	provisioners machines.MachineProvisioners,
	templates api.Templates,
	credentials api.Credentials) api.Swarms {

	return &swarms{
		provisioners: provisioners,
		templates:    templates,
		credentials:  credentials,
	}
}

func (s swarms) ListIDs() ([]api.SwarmID, *api.Error) {
	return nil, api.UnknownError(errors.New("Not implemented"))
}

func (s swarms) Get(id api.SwarmID) (interface{}, *api.Error) {
	return nil, api.UnknownError(errors.New("Not implemented"))
}

func (s swarms) Create(
	provisionerName string,
	credentialsName string,
	templateName string,
	input io.Reader,
	codec api.Codec) (<-chan interface{}, *api.Error) {

	return nil, api.UnknownError(errors.New("Not implemented"))
}

func (s swarms) Delete(credentialsName string, swarm api.SwarmID) (<-chan interface{}, *api.Error) {
	return nil, api.UnknownError(errors.New("Not implemented"))
}

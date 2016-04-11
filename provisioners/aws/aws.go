package aws

import (
	api "github.com/docker/libmachete"
)

type aws struct {
}

func init() {
	impl := &aws{}

	api.Register("aws", impl)
}

func (provisioner *aws) Create(request api.CreateRequest) (<-chan api.CreateEvent, error) {
	return nil, nil
}

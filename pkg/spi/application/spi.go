package application

import (
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/types"
)

// InterfaceSpec is the current name and version of the Application API.
var InterfaceSpec = spi.InterfaceSpec{
	Name:    "Application",
	Version: "0.1.0",
}

// Health is an indication of whether the Application is functioning properly.
type Health int

const (
	// Unknown indicates that the Health cannot currently be confirmed.
	Unknown Health = iota

	// Healthy indicates that the Application is confirmed to be functioning.
	Healthy

	// Unhealthy indicates that the Application is confirmed to not be functioning properly.
	Unhealthy
)

// Plugin defines custom behavior for what runs on flavors/events.
type Plugin interface {

	// Validate checks whether the helper can support a configuration.
	Validate(applicationProperties *types.Any) error

	// Healthy determines the Health of this Application on a flavor/enent.
	Healthy(applicationProperties *types.Any) (Health, error)

	Update(message *Message) error
}

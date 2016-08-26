package controller

// Info is the data that a driver is expected to return when responding to the /v1/info call
type Info struct {

	// Name is the driver friendly name
	Name string

	// DriverName is a system name (unlike the user friendly name) of the driver
	DriverName string

	// DriverType is the name used in the RPC url call.  For example, 'scaler' in /v1/scaler.Start
	DriverType string

	// Version is the version string
	Version string

	// Revision is the revision
	Revision string

	// Description friendly description
	Description string

	// Namespace used in the swim config
	Namespace string

	// Image is the container image
	Image string

	// Capabilities is a list of capabilities such as 'bootstrap', 'runtime', 'teardown'
	Capabilities []string
}

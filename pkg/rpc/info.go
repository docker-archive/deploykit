package rpc

const (
	// APIURL is the well-known HTTP GET endpoint that retrieves description of the plugin's interfaces.
	APIURL = "/info/api.json"

	// FunctionsURL exposes the templates functions that are available via this plugin
	FunctionsURL = "/info/functions.json"
)

// InputExample is the interface implemented by the rpc implementations for
// group, instance, and flavor to set example input using custom/ vendored data types.
type InputExample interface {

	// SetExampleProperties updates the parameter with example properties.
	// The request param must be a pointer
	SetExampleProperties(request interface{})
}

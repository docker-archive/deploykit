package rpc

const (
	// InfoURL is the well-known HTTP GET endpoint that retrieves description of the plugin's interfaces.
	InfoURL = "/info/api.json"
)

// InputExample is the interface implemented by the rpc implementations for
// group, instance, and flavor to set example input using custom/ vendored data types.
type InputExample interface {

	// SetExampleProperties updates the parameter with example properties.
	// The request param must be a pointer
	SetExampleProperties(request interface{})
}

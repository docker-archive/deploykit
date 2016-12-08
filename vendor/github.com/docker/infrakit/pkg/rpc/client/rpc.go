package client

// Client allows execution of RPCs.
type Client interface {

	// Call invokes an RPC method with an argument and a pointer to a result that will hold the return value.
	Call(method string, arg interface{}, result interface{}) error
}

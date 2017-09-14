package lb

// ListenerClient is a client struct for the load balancing listener
type ListenerClient struct {
	Client *LBClient
}

// ListenerClient provides a Client interface for load balancing listener API calls
func (c *LBClient) ListenerClient() *ListenerClient {
	return &ListenerClient{
		Client: c,
	}
}

type Listener struct {
	BackendName string `json:"defaultBackendSetName"`
	Name        string `json:"name"`
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"` // HTTP or TCP
	SSLConfig   string `json:"sslConfiguration"`
}

type SSLConfiguration struct {
	CertName    string `json:"certificateName"`
	VerifyDepth int    `json:"verifyDepth"`
	VerifyPeer  bool   `json:"verifyPeerCertificate"`
}

// When creating a Listener a Backend Set is needed

func (l *ListenerClient) CreateListener(listener *Listener) {
	// POST loadBalancers/{loadBalancerId}/listeners
}

func (l *ListenerClient) UpdateListener(listener *Listener) {
	// PUT loadBalancers/{loadBalancerId}/listeners/{listenerName}

}

func (l *ListenerClient) DeleteListener(listener *Listener) {
	// DELETE loadBalancers/{loadBalancerId}/listeners/{listenerName}
}

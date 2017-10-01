package plugin

// Options contain the static configs for the plugin to start up, e.g. credentials, etc.
type Options struct {
	ResourceGroup  string
	SubscriptionID string
	Token          string
	Namespace      map[string]string
}

// OAuthToken implements the OAuthTokenProvider interface
func (o Options) OAuthToken() string {
	return o.Token
}

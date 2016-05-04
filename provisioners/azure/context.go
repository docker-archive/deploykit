package azure

import (
	"github.com/Azure/go-autorest/autorest/azure"
	"golang.org/x/net/context"
)

// See http://blog.golang.org/context on Context pattern
// and http://blog.golang.org/context/userip/userip.go for derived contexts

var (
	environments = map[string]azure.Environment{
		azure.PublicCloud.Name:       azure.PublicCloud,
		azure.USGovernmentCloud.Name: azure.USGovernmentCloud,
		azure.ChinaCloud.Name:        azure.ChinaCloud,
	}
)

type contextKey int

const (
	environmentKey contextKey = iota
	subscriptionIDKey
	oauthClientIDKey
)

// BuildContext returns a context that's properly configured with the required context data.
func BuildContext(parent context.Context, clientID, subscriptionID, environment string) context.Context {
	return WithEnvironment(WithSubscriptionID(WithOAuthClientID(parent, clientID), subscriptionID), environment)
}

// WithEnvironment returns a new context given a parent context and the environment.
// If environment is not a known string value, then the context will not be set with the environment
// and errors downstream will happen.
func WithEnvironment(parent context.Context, environment string) context.Context {
	if env, exists := environments[environment]; exists {
		return context.WithValue(parent, environmentKey, env)
	}
	return parent
}

// EnvironmentFromContext returns the Azure environment from the request context.
func EnvironmentFromContext(ctx context.Context) (azure.Environment, bool) {
	v, ok := ctx.Value(environmentKey).(azure.Environment)
	return v, ok
}

// WithSubscriptionID adds the subscription id in the current context.
func WithSubscriptionID(parent context.Context, subscriptionID string) context.Context {
	return context.WithValue(parent, subscriptionIDKey, subscriptionID)
}

// SubscriptionIDFromContext retrieves the subscription id from the context
func SubscriptionIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(subscriptionIDKey).(string)
	return v, ok
}

// WithOAuthClientID populates the API client id.  This is the id that the application that uses this lib
// must provide after it registers with the Azure service as a verified application.
func WithOAuthClientID(parent context.Context, clientID string) context.Context {
	return context.WithValue(parent, oauthClientIDKey, clientID)
}

// OAuthClientIDFromContext retrieves the OAuth client id from the context
func OAuthClientIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(oauthClientIDKey).(string)
	return v, ok
}

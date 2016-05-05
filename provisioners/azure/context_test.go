package azure

import (
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"testing"
)

func TestInvalidEnvironment(t *testing.T) {
	environment := "bogus"
	ctx := WithEnvironment(context.Background(), environment)
	_, ok := EnvironmentFromContext(ctx)
	require.False(t, ok)
}

func TestBuildContext(t *testing.T) {

	clientID := "clientID"
	subscriptionID := "subscriptionID"
	environment := azure.PublicCloud

	ctx := BuildContext(context.Background(), clientID, subscriptionID, environment.Name)
	require.NotNil(t, ctx)

	c, ok := OAuthClientIDFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, clientID, c)

	s, ok := SubscriptionIDFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, subscriptionID, s)

	e, ok := EnvironmentFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, environment, e)
}

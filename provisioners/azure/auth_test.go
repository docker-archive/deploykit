package azure

import (
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"testing"
)

func TestInvalidEmptyCredential(t *testing.T) {
	cr := NewCredential()
	require.Len(t, cr.(*credential).AccessToken, 0)
	require.Len(t, cr.(*credential).RefreshToken, 0)

	clientID := "clientID"
	subscriptionID := "subscriptionID"
	environment := azure.PublicCloud
	ctx := BuildContext(context.Background(), clientID, subscriptionID, environment.Name)
	require.Error(t, cr.Validate(ctx))
}

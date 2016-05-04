package azure

import (
	"github.com/docker/libmachete/provisioners/azure"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"os"
	"testing"
)

func requireEnvVar(t *testing.T, varName string) string {
	value := os.Getenv(varName)
	require.NotEmpty(t, value, varName+" environment variable must be set")
	return value
}

func TestAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode.")
		return
	}

	/*
		From Docker Machine, for example:
		environment := "AzurePublicCloud"
		subscriptionID := "71be52f3-def6-4c7c-85b4-c1053ba503b2"
		clientID := "637ddaba-219b-43b8-bf19-8cea500cf273"
	*/

	clientID := requireEnvVar(t, "AZURE_CLIENT_ID")
	subscriptionID := requireEnvVar(t, "AZURE_SUBSCRIPTION_ID")
	environment := requireEnvVar(t, "AZURE_ENVIRONMENT")

	// The responsibility of the provision writer to properly configure the context
	ctx := azure.BuildContext(context.Background(), clientID, subscriptionID, environment)

	// Get a blank object
	cred := azure.NewCredential()

	// Typically here the framework calling the provisioner will unmarshal user's input onto
	// the structure here.

	err := cred.Validate(ctx)
	require.NotNil(t, err)

	t.Log("err=", err)

	if err != nil {

		t.Log("Must authenticate")
		err := cred.Authenticate(ctx)

		require.Nil(t, err)

		// Ok to persist the cred here for future use
		buff, err := yaml.Marshal(cred)
		require.Nil(t, err)
		t.Log(string(buff))

		// This should hold now that we've authenticated.
		t.Log("Validate the authenticated credential")
		err = cred.Validate(ctx)
		require.Nil(t, err)

		t.Log("Should also be able to refresh")
		err = cred.Refresh(ctx)
		require.Nil(t, err)

		// We can persist this again somewhere else.
		buff, err = yaml.Marshal(cred)
		require.Nil(t, err)
		t.Log(string(buff))

		// TODO -- check with MSFT on whether after the refresh a new auth flow need to be preform again
		// since the refresh_token can be set to empty string here.
	}
}

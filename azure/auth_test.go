package azure

import (
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	"os"
	"testing"
)

func requireEnvVar(t *testing.T, varName string) string {
	value := os.Getenv(varName)
	require.NotEmpty(t, value, varName+" environment variable must be set")
	return value
}

func TestADAppIDSecretFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode.")
		return
	}

	appID := requireEnvVar(t, "AD_APP_ID")
	appSecret := requireEnvVar(t, "AD_APP_SECRET")

	subscriptionID := requireEnvVar(t, "AZURE_SUBSCRIPTION_ID")
	environment := requireEnvVar(t, "AZURE_ENVIRONMENT")

	opt := Options{
		Environment:    environment,
		SubscriptionID: subscriptionID,
		ADClientID:     appID,
		ADClientSecret: appSecret,
	}

	// Get a blank object
	cred := NewCredential()

	// Typically here the framework calling the provisioner will unmarshal user's input onto
	// the structure here.

	err := cred.Validate(opt)
	if err != nil {

		t.Log("Must authenticate")
		err := cred.Authenticate(opt)

		require.NoError(t, err)

		// Ok to persist the cred here for future use
		buff, err := yaml.Marshal(cred)
		require.NoError(t, err)
		t.Log(string(buff))
	}
}

func TestOAuthDeviceFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode.")
		return
	}

	if requireEnvVar(t, "TEST_AD_CLIENT_AUTH") == "true" {
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

	opt := Options{
		Environment:    environment,
		SubscriptionID: subscriptionID,
		OAuthClientID:  clientID,
	}

	// Get a blank object
	cred := NewCredential()

	// Typically here the framework calling the provisioner will unmarshal user's input onto
	// the structure here.

	err := cred.Validate(opt)
	if err != nil {

		t.Log("Must authenticate")
		err := cred.Authenticate(opt)

		require.NoError(t, err)

		// Ok to persist the cred here for future use
		buff, err := yaml.Marshal(cred)
		require.NoError(t, err)
		t.Log(string(buff))

		// This should hold now that we've authenticated.
		t.Log("Validate the authenticated credential")
		err = cred.Validate(opt)
		require.NoError(t, err)

		t.Log("Should also be able to refresh")
		err = cred.Refresh(opt)
		require.NoError(t, err)

		// We can persist this again somewhere else.
		buff, err = yaml.Marshal(cred)
		require.NoError(t, err)
		t.Log(string(buff))

		// TODO -- check with MSFT on whether after the refresh a new auth flow need to be preform again
		// since the refresh_token can be set to empty string here.
	}
}

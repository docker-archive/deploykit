package azure

import (
	"fmt"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	log "github.com/Sirupsen/logrus"
	"net/http"
	"regexp"
)

var (
	environments = map[string]azure.Environment{
		azure.PublicCloud.Name:       azure.PublicCloud,
		azure.USGovernmentCloud.Name: azure.USGovernmentCloud,
		azure.ChinaCloud.Name:        azure.ChinaCloud,
	}

	// DefaultEnvironment assumed environment - for use by CLI or other clients to avoid import azure packages
	DefaultEnvironment = azure.PublicCloud.Name
)

// NewCredential allocates a credential
func NewCredential() *Credential {
	return new(Credential)
}

// Credential for azure
type Credential struct {
	azure.Token
	authorizer autorest.Authorizer
}

func (a Credential) loadSPT(opt Options) (*azure.ServicePrincipalToken, error) {
	env, ok := environments[opt.Environment]
	if !ok {
		return nil, fmt.Errorf("No valid environment")
	}

	tenantID, err := getTenantID(env, opt.SubscriptionID)
	if err != nil {
		return nil, err
	}

	oauthCfg, err := env.OAuthConfigForTenant(tenantID)
	if err != nil {
		return nil, err
	}

	return azure.NewServicePrincipalTokenFromManualToken(*oauthCfg,
		opt.OAuthClientID, env.ServiceManagementEndpoint, a.Token)
}

// Validate performs validation on the credential
func (a Credential) Validate(opt Options) error {
	if len(a.AccessToken) == 0 && len(a.RefreshToken) == 0 {
		return fmt.Errorf("no token")
	}

	env, ok := environments[opt.Environment]
	if !ok {
		return fmt.Errorf("No valid environment")
	}

	spt, err := a.loadSPT(opt)
	if err != nil {
		return err
	}
	return validateToken(env, spt)
}

// Authenticate performs authentication
func (a *Credential) Authenticate(opt Options) error {
	env, ok := environments[opt.Environment]
	if !ok {
		return fmt.Errorf("No valid environment")
	}

	tenantID, err := getTenantID(env, opt.SubscriptionID)
	if err != nil {
		return err
	}

	oauthCfg, err := env.OAuthConfigForTenant(tenantID)
	if err != nil {
		return err
	}

	var spt *azure.ServicePrincipalToken

	if opt.ADClientID != "" && opt.ADClientSecret != "" {
		spt, err = azure.NewServicePrincipalToken(
			*oauthCfg,
			opt.ADClientID,
			opt.ADClientSecret,
			env.ServiceManagementEndpoint)
		if err != nil {
			return err
		}
		a.authorizer = spt
		return nil
	}

	spt, err = tokenFromDeviceFlow(*oauthCfg, opt.OAuthClientID, env.ServiceManagementEndpoint)
	if err != nil {
		return err
	}
	if len(spt.AccessToken) > 0 && len(spt.RefreshToken) > 0 {
		a.Token = spt.Token
		return nil
	}

	return fmt.Errorf("not-authorized")
}

// Refresh refreshes the oauth token
func (a *Credential) Refresh(opt Options) error {
	spt, err := a.loadSPT(opt)
	if err != nil {
		return err
	}
	err = spt.Refresh()
	if err != nil {
		return err
	}
	a.Token = spt.Token
	return nil
}

// validateToken makes a call to Azure SDK with given token, essentially making
// sure if the access_token valid, if not it uses SDK’s functionality to
// automatically refresh the token using refresh_token (which might have
// expired). This check is essentially to make sure refresh_token is good.
func validateToken(env azure.Environment, token *azure.ServicePrincipalToken) error {
	c := subscriptionsClient(env.ResourceManagerEndpoint)
	c.Authorizer = token
	_, err := c.List()
	if err != nil {
		return fmt.Errorf("Token validity check failed: %v", err)
	}
	return nil
}

// getTenantID figures out the AAD tenant ID of the subscription by making an
// unauthenticated request to the Get Subscription Details endpoint and parses
// the value from WWW-Authenticate header.
func getTenantID(env azure.Environment, subscriptionID string) (string, error) {
	const hdrKey = "WWW-Authenticate"
	c := subscriptionsClient(env.ResourceManagerEndpoint)

	// we expect this request to fail (err != nil), but we are only interested
	// in headers, so surface the error if the Response is not present (i.e.
	// network error etc)
	subs, err := c.Get(subscriptionID)
	if subs.Response.Response == nil {
		return "", fmt.Errorf("Request failed: %v", err)
	}

	// Expecting 401 StatusUnauthorized here, just read the header
	if subs.StatusCode != http.StatusUnauthorized {
		return "", fmt.Errorf("Unexpected response from Get Subscription: %v", err)
	}
	hdr := subs.Header.Get(hdrKey)
	if hdr == "" {
		return "", fmt.Errorf("Header %v not found in Get Subscription response", hdrKey)
	}

	// Example value for hdr:
	//   Bearer authorization_uri="https://login.windows.net/996fe9d1-6171-40aa-945b-4c64b63bf655", error="invalid_token", error_description="The authentication failed because of missing 'Authorization' header."
	r := regexp.MustCompile(`authorization_uri=".*/([0-9a-f\-]+)"`)
	m := r.FindStringSubmatch(hdr)
	if m == nil {
		return "", fmt.Errorf("Could not find the tenant ID in header: %s %q", hdrKey, hdr)
	}
	return m[1], nil
}

// tokenFromDeviceFlow prints a message to the screen for user to take action to
// consent application on a browser and in the meanwhile the authentication
// endpoint is polled until user gives consent, denies or the flow times out.
// Returned token must be saved.
func tokenFromDeviceFlow(oauthCfg azure.OAuthConfig, clientID, resource string) (*azure.ServicePrincipalToken, error) {
	cl := oauthClient()
	deviceCode, err := azure.InitiateDeviceAuth(&cl, oauthCfg, clientID, resource)
	if err != nil {
		return nil, fmt.Errorf("Failed to start device auth: %v", err)
	}
	log.Debugln("Retrieved device code.", deviceCode)

	// Example message: “To sign in, open https://aka.ms/devicelogin and enter
	// the code 0000000 to authenticate.”
	log.Infof("Microsoft Azure: %s", to.String(deviceCode.Message))

	token, err := azure.WaitForUserCompletion(&cl, deviceCode)
	if err != nil {
		return nil, fmt.Errorf("Failed to complete device auth: %v", err)
	}

	spt, err := azure.NewServicePrincipalTokenFromManualToken(oauthCfg, clientID, resource, *token)
	if err != nil {
		return nil, fmt.Errorf("Error constructing service principal token: %v", err)
	}
	return spt, nil
}

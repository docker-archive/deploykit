package compute

import (
	"fmt"
	"net/http"
	"time"
)

// AuthenticationReq represents the body of an authentication request.
type AuthenticationReq struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

// Authenticate authenticates this client with the service, returning an AuthenticatedClient which will re-use the
// retrieved authentication token in subsequent requests.
func (c *Client) Authenticate() (*AuthenticatedClient, error) {
	authenticationCookie, err := c.getAuthenticationCookie()

	if err != nil {
		return nil, err
	}

	return &AuthenticatedClient{
		Client:               c,
		authenticationCookie: authenticationCookie,
		cookieIssued:         time.Now(),
	}, nil
}

func (c *AuthenticatedClient) refreshAuthenticationCookie() error {
	cookie, err := c.getAuthenticationCookie()
	if err != nil {
		return err
	}
	c.authenticationCookie = cookie
	c.cookieIssued = time.Now()
	return nil
}

func (c *Client) getAuthenticationCookie() (*http.Cookie, error) {
	req, err := c.newPostRequest(
		"/authenticate/",
		AuthenticationReq{
			User:     c.computeUserName(),
			Password: c.password,
		})

	if err != nil {
		return nil, err
	}

	rsp, err := c.requestAndCheckStatus("authenticate", req)

	if err != nil {
		return nil, err
	}

	if len(rsp.Cookies()) == 0 {
		return nil, fmt.Errorf("No authentication cookie found in response %#v", rsp)
	}

	return rsp.Cookies()[0], nil
}

// AuthenticatedClient is a compute Client equipped with an authentication cookie.
type AuthenticatedClient struct {
	*Client
	authenticationCookie *http.Cookie
	cookieIssued         time.Time
}

func (c *AuthenticatedClient) setAuthenticationCookie(req *http.Request) error {
	if time.Since(c.cookieIssued).Minutes() > 25 {
		if err := c.refreshAuthenticationCookie(); err != nil {
			return err
		}
	}

	req.AddCookie(c.authenticationCookie)

	return nil
}

func (c *AuthenticatedClient) newAuthenticatedRequest(builder func() (*http.Request, error)) (*http.Request, error) {
	req, err := builder()
	if err != nil {
		return nil, err
	}

	if err = c.setAuthenticationCookie(req); err != nil {
		return nil, err
	}

	return req, nil
}

func (c *AuthenticatedClient) newAuthenticatedPostRequest(path string, body interface{}) (*http.Request, error) {
	return c.newAuthenticatedRequest(func() (*http.Request, error) { return c.newPostRequest(path, body) })
}

func (c *AuthenticatedClient) newAuthenticatedPutRequest(path string, body interface{}) (*http.Request, error) {
	return c.newAuthenticatedRequest(func() (*http.Request, error) { return c.newPutRequest(path, body) })
}

func (c *AuthenticatedClient) newAuthenticatedGetRequest(path string) (*http.Request, error) {
	return c.newAuthenticatedRequest(func() (*http.Request, error) { return c.newGetRequest(path) })
}

func (c *AuthenticatedClient) newAuthenticatedDeleteRequest(path string) (*http.Request, error) {
	return c.newAuthenticatedRequest(func() (*http.Request, error) { return c.newDeleteRequest(path) })
}

package compute

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// Test that the client can obtain an authentication cookie from the authentication endpoint.
func TestObtainAuthenticationCookie(t *testing.T) {
	identityDomain := "opencredodev"
	username := "user@test.com"
	password := "p4ssw0rd"

	authCookie := http.Cookie{
		Name:  "testAuthCookie",
		Value: "cookie value",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("Wrong HTTP method %s, expected POST", r.Method)
		}

		expectedPath := "/authenticate/"
		if r.URL.Path != expectedPath {
			t.Fatalf("Wrong HTTP URL %v, expected %v", r.URL, expectedPath)
		}

		req := &AuthenticationReq{}
		unmarshalRequestBody(t, r, req)

		expectedUsername := fmt.Sprintf("/Compute-%s/%s", identityDomain, username)
		if req.User != expectedUsername {
			t.Fatalf("Wrong username %s, expected %s", req.User, expectedUsername)
		}

		if req.Password != password {
			t.Fatalf("Wrong password %s, expected %s", req.Password, password)
		}

		http.SetCookie(w, &authCookie)
		w.WriteHeader(200)
	}))

	defer server.Close()

	endpoint, _ := url.Parse(server.URL)

	client := NewComputeClient(
		identityDomain,
		username,
		password,
		endpoint,
	)

	authenticatedClient, err := client.Authenticate()
	if err != nil {
		t.Fatalf("Authentication failed: %s", err)
	}

	if authenticatedClient.authenticationCookie == nil {
		t.Fatal("Authentication cookie not set")
	}
}

// Test that the authenticating client sends the authentication cookie with all requests to the API.
func TestAuthenticationCookieSentInRequestsFromAuthenticatedClient(t *testing.T) {
	server := newAuthenticatingServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.Cookies()) == 0 {
			t.Fatal("No cookie sent with request")
		}

		w.WriteHeader(200)
	}))

	defer server.Close()

	endpoint, _ := url.Parse(server.URL)

	client := NewComputeClient(
		"mydomain",
		"user",
		"password",
		endpoint,
	)

	authClient, err := client.Authenticate()
	if err != nil {
		t.Fatalf("Authentication failed: %s", err)
	}

	_, err = authClient.newAuthenticatedGetRequest("foo")
	if err != nil {
		t.Fatalf("Authenticatde request failed: %s", err)
	}
}

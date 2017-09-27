package monorail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-openapi/runtime"
	rc "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/spiegela/gorackhd/client"
	"github.com/spiegela/gorackhd/jwt"
)

// Monorail type wraps RackHD Monorail client with methods to enable mockable interfaces
type Monorail struct {
	Client   *client.Monorail
	Endpoint *url.URL
}

// Login authenticates a username and password against the Monorail API to obtain a
// client token, which can then be used to authorize future requests
func (m *Monorail) Login(user string, pass string) (runtime.ClientAuthInfoWriter, error) {
	body := fmt.Sprintf("{\"username\": \"%s\", \"password\": \"%s\"}", user, pass)
	url := fmt.Sprintf("%s://%s/login", m.Endpoint.Scheme, m.Endpoint.Host)
	buff := bytes.NewBufferString(body)

	resp, err := http.Post(url, "application/json", buff)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var retval map[string]string
	if err == nil {
		json.NewDecoder(resp.Body).Decode(&retval)
	}

	return jwt.BearerJWTToken(retval["token"]), err
}

// New instantiates a new Monorail client instance
func New(endpoint string) *Monorail {
	u, err := url.Parse(endpoint)
	if err != nil {
		panic(err)
	}
	transport := rc.New(u.Host, "/api/2.0/", []string{u.Scheme})
	monorail := client.New(transport, strfmt.Default)
	return &Monorail{Client: monorail, Endpoint: u}
}

// Nodes provides a RackHD Nodes client
func (m *Monorail) Nodes() NodeIface {
	return m.Client.Nodes
}

// Skus provides a RackHD Nodes client
func (m *Monorail) Skus() SkuIface {
	return m.Client.Skus
}

// Tags provides a RackHD Tags client
func (m *Monorail) Tags() TagIface {
	return m.Client.Tags
}

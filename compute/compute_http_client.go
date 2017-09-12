package compute

import (
	"crypto/rsa"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/spacemonkeygo/httpsig"
)

// APIClient represents the struct for basic api calls
type APIClient struct {
	apiEndpoint   *url.URL
	apiKey        string
	apiPrivateKey *rsa.PrivateKey
	httpClient    *http.Client
}

var headersToSign = []string{"(request-target)", "date", "host"}

// Sign request with POST/PUT:
// Authorization: Signature version="1",keyId="<TENANCY OCID>/<USER OCID>/<KEY FINGERPRINT>",algorithm="rsa-sha256",headers="(request-target) date host x-content-sha256 content-type content-length",signature="Base64(RSA-SHA256(SIGNING STRING))"
// Sign request with GET:
// Authorization: Signature version="1",keyId="<TENANCY OCID>/<USER OCID>/<KEY FINGERPRINT>",algorithm="rsa-sha256",headers="(request-target) date host",signature="Base64(RSA-SHA256(SIGNING STRING))"

func (c *APIClient) signAuthHeader(req *http.Request) {
	if strings.HasPrefix(req.Method, "P") {

		headersToSign = append(headersToSign, "x-content-sha256", "content-type", "content-length")
	}
	signer := httpsig.NewRSASHA256Signer(c.apiKey, c.apiPrivateKey, headersToSign)
	err := signer.Sign(req)
	if err != nil {
		logrus.Fatalf("Could not sign request: %s", err)
	}
	logrus.Infof("Length: %v", req.Header.Get("Content-Length"))
	req.Header.Set("Authorization", strings.Replace(req.Header.Get("Authorization"), "Signature", "Signature version=\"1\",", 1))

	return
}

// Get request a resource from Oracle
func (c *APIClient) Get(path string) (*http.Response, error) {
	// Parse URL Path
	urlPath, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	logrus.Infof("Making URL request to: %s", c.formatURL(urlPath))
	// Create Request
	req, err := http.NewRequest("GET", c.formatURL(urlPath), nil)
	if err != nil {
		return nil, err
	}
	c.signAuthHeader(req)
	logrus.Infof("Auth: %s", req.Header.Get("Authorization"))

	return c.httpClient.Do(req)
}

func (c *APIClient) formatURL(urlPath *url.URL) string {
	urlEndpoint := c.apiEndpoint
	urlEndpoint.Path = path.Join(urlEndpoint.Path, urlPath.Path)
	urlEndpoint.RawQuery = urlPath.RawQuery
	return urlEndpoint.String()
}

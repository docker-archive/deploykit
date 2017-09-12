package compute

import (
	"crypto/rsa"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

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

var headersToSign = []string{"date", "(request-target)", "host"}

// Sign request with POST/PUT:
// Authorization: Signature version="1",keyId="<TENANCY OCID>/<USER OCID>/<KEY FINGERPRINT>",algorithm="rsa-sha256",headers="(request-target) date host x-content-sha256 content-type content-length",signature="Base64(RSA-SHA256(SIGNING STRING))"
// Sign request with GET:
// Authorization: Signature version="1",keyId="<TENANCY OCID>/<USER OCID>/<KEY FINGERPRINT>",algorithm="rsa-sha256",headers="(request-target) date host",signature="Base64(RSA-SHA256(SIGNING STRING))"

func (c *APIClient) signAuthHeader(req *http.Request) {
	// Add missing defaults
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if req.Header.Get("Host") == "" {
		req.Header.Set("Host", c.apiEndpoint.Hostname())
	}
	if req.Header.Get("Date") == "" {
		t := time.Now()
		req.Header.Set("Date", t.Format(time.RFC1123))
	}

	if strings.HasPrefix(req.Method, "P") {
		headersToSign = append(headersToSign, "x-content-sha256", "content-type", "content-length")
	}
	signer := httpsig.NewRSASHA256Signer(c.apiKey, c.apiPrivateKey, headersToSign)
	err := signer.Sign(req)
	if err != nil {
		logrus.Fatalf("Could not sign request: %s", err)
	}

	return
}

// Get request a resource from Oracle
func (c *APIClient) Get(path string) (*http.Response, error) {
	// Parse URL Path
	urlPath, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	// Create Request
	req, err := http.NewRequest("GET", c.formatURL(urlPath), nil)
	if err != nil {
		return nil, err
	}

	c.signAuthHeader(req)
	logrus.Debug("Auth: ", req.Header)
	logrus.Info("Request URL: ", req.URL.String())
	return c.httpClient.Do(req)
}

func (c *APIClient) formatURL(urlPath *url.URL) string {
	urlEndpoint := c.apiEndpoint
	urlEndpoint.Path = path.Join(urlEndpoint.Path, urlPath.Path)
	urlEndpoint.RawQuery = urlPath.RawQuery
	return urlEndpoint.String()
}

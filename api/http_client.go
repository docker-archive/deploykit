package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/spacemonkeygo/httpsig"
)

var headersToSign = []string{"date", "(request-target)", "host"}

// Sign request with POST/PUT:
// Authorization: Signature version="1",keyId="<TENANCY OCID>/<USER OCID>/<KEY FINGERPRINT>",algorithm="rsa-sha256",headers="(request-target) date host x-content-sha256 content-type content-length",signature="Base64(RSA-SHA256(SIGNING STRING))"
// Sign request with GET:
// Authorization: Signature version="1",keyId="<TENANCY OCID>/<USER OCID>/<KEY FINGERPRINT>",algorithm="rsa-sha256",headers="(request-target) date host",signature="Base64(RSA-SHA256(SIGNING STRING))"

func (c *Client) signAuthHeader(req *http.Request, body []byte) {
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
		// if len(body) > 0 {
		// 	req.Header.Set("Content-Type", "application/oracle-compute-v3+json")
		// }
		headersToSign = append(headersToSign, "x-content-sha256", "content-type", "content-length")
		bodyHash := sha256.Sum256(body)
		req.Header.Set("Content-Length", string(len(body)))
		req.Header.Set("X-Content-Sha256", fmt.Sprintf("%x", bodyHash))
	}

	signer := httpsig.NewRSASHA256Signer(c.apiKey, c.apiPrivateKey, headersToSign)
	err := signer.Sign(req)
	if err != nil {
		logrus.Fatalf("Could not sign request: %s", err)
	}

	return
}

func (c *Client) formatURL(urlPath *url.URL) string {
	urlEndpoint := *c.apiEndpoint
	urlEndpoint.Path = path.Join(urlEndpoint.Path, urlPath.Path)
	urlEndpoint.RawQuery = urlPath.RawQuery
	return urlEndpoint.String()
}

// Request request a resource from Oracle
func (c *Client) Request(method string, path string, body interface{}) (*http.Response, error) {
	// Parse URL Path
	urlPath, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	// Marshall request body
	var requestBody io.ReadSeeker
	var marshaled []byte
	if body != nil {
		marshaled, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
		requestBody = bytes.NewReader(marshaled)
	}

	// Create Request
	req, err := http.NewRequest(method, c.formatURL(urlPath), requestBody)
	if err != nil {
		return nil, err
	}

	c.signAuthHeader(req, marshaled)
	logrus.Debug("Auth: ", req.Header)
	logrus.Debug("Request URL: ", req.URL.String())
	return c.httpClient.Do(req)
}

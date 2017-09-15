package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/spacemonkeygo/httpsig"
)

// Signing details: https://docs.us-phoenix-1.oraclecloud.com/Content/API/Concepts/signingrequests.htm

var headersToSign = []string{"date", "(request-target)", "host"}

func (c *Client) signAuthHeader(req *http.Request, body []byte) {
	// Add missing defaults
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if req.Header.Get("Host") == "" {
		req.Header.Set("Host", req.URL.Hostname())
	}
	if req.Header.Get("Date") == "" {
		t := time.Now()
		req.Header.Set("Date", t.Format(time.RFC1123))
	}

	if strings.HasPrefix(req.Method, "P") {
		headersToSign = append(headersToSign, "x-content-sha256", "content-type", "content-length")
		bodyHash := sha256.Sum256(body)
		req.Header.Set("Content-Length", strconv.Itoa(len(body)))
		req.Header.Set("X-Content-Sha256", base64.StdEncoding.EncodeToString(bodyHash[:]))
	}

	signer := httpsig.NewRSASHA256Signer(c.apiKey, c.apiPrivateKey, headersToSign)
	err := signer.Sign(req)
	if err != nil {
		logrus.Fatalf("Could not sign request: %s", err)
	}

	return
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
	req, err := http.NewRequest(method, c.buildAPIEndpoint(urlPath), requestBody)
	if err != nil {
		return nil, err
	}

	c.signAuthHeader(req, marshaled)
	logrus.Debug("Auth: ", req.Header)
	logrus.Debug("Request URL: ", req.URL.String())
	return c.httpClient.Do(req)
}

// getAPIEndpoint builds the API endpoint given a URL
func (c *Client) buildAPIEndpoint(urlPath *url.URL) string {
	urlEndpoint, err := url.Parse(fmt.Sprintf(apiEndpointFormat, c.apiRegion, c.APIVersion))
	if err != nil {
		log.Fatalf("Error parsing API Endpoint: %s", err)
	}
	urlEndpoint.Path = path.Join(urlEndpoint.Path, urlPath.Path)
	urlEndpoint.RawQuery = urlPath.RawQuery
	return urlEndpoint.String()
}

package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/spacemonkeygo/httpsig"
)

// Signing details: https://docs.us-phoenix-1.oraclecloud.com/Content/API/Concepts/signingrequests.htm

var headersConst = []string{"date", "(request-target)", "host"}

// Clienter is the client interface for all requests
type Clienter interface {
	Request(string, string, interface{}) (*http.Response, error)
}

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
	headersToSign := headersConst
	if strings.HasPrefix(req.Method, "P") {

		headersToSign = append(headersConst, "x-content-sha256", "content-type", "content-length")
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
func (c *Client) Request(method string, reqURL string, body interface{}) (*http.Response, error) {
	// Parse URL Path
	urlPath, err := url.Parse(reqURL)
	if err != nil {
		return nil, err
	}

	// Marshall request body
	var requestBody io.ReadSeeker
	var marshaled []byte
	if body != nil {
		marshaled, err = json.Marshal(body)
		logrus.Debug("Body: ", string(marshaled))
		if err != nil {
			return nil, err
		}
		requestBody = bytes.NewReader(marshaled)
	}

	// Create Request
	req, err := http.NewRequest(method, urlPath.String(), requestBody)
	if err != nil {
		return nil, err
	}

	c.signAuthHeader(req, marshaled)
	logrus.Debug("Auth: ", req.Header)
	logrus.Debug("Request URL: ", req.URL.String(), " Method: ", method)
	return c.httpClient.Do(req)
}

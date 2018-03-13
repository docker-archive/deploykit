package docker

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"

	"github.com/docker/docker/client"
	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
	logutil "github.com/docker/infrakit/pkg/log"
)

var (
	// ClientVersion is the Docker client API version to use when connecting to Docker
	// See Makefile targets that may set this at build time.
	ClientVersion = "1.24"

	log    = logutil.New("module", "util/docker")
	debugV = logutil.V(500)
)

// ConnectInfo holds the connection parameters for connecting to a Docker engine to get join tokens, etc.
type ConnectInfo struct {
	Host string
	TLS  *tlsconfig.Options
}

// APIClientCloser is a closeable API client.
type APIClientCloser interface {
	io.Closer
	client.CommonAPIClient
}

// NewClient creates a new API client.
func NewClient(host string, tls *tlsconfig.Options) (APIClientCloser, error) {
	tlsOptions := tls
	if tls == nil || tls.KeyFile == "" || tls.CAFile == "" || tls.CertFile == "" {
		// The api doesn't like it when you pass in not nil but with zero field values...
		tlsOptions = nil
	}
	customHeaders := map[string]string{
		"User-Agent": clientUserAgent(),
	}
	verStr := ClientVersion
	if tmpStr := os.Getenv("DOCKER_API_VERSION"); tmpStr != "" {
		verStr = tmpStr
	}
	httpClient, err := newHTTPClient(host, tlsOptions)
	if err != nil {
		return &client.Client{}, err
	}
	return client.NewClient(host, verStr, httpClient, customHeaders)
}

func newHTTPClient(host string, tlsOptions *tlsconfig.Options) (*http.Client, error) {

	var config *tls.Config
	var err error

	if tlsOptions != nil {
		config, err = tlsconfig.Client(*tlsOptions)
		if err != nil {
			return nil, err
		}
		log.Debug("TLS config", "config", config)
	}
	tr := &http.Transport{
		TLSClientConfig: config,
	}
	proto, addr, _, err := client.ParseHost(host)
	if err != nil {
		return nil, err
	}
	sockets.ConfigureTransport(tr, proto, addr)
	return &http.Client{
		Transport: tr,
	}, nil
}

func clientUserAgent() string {
	return fmt.Sprintf("Docker-Client/%s (%s)", ClientVersion, runtime.GOOS)
}

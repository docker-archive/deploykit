package loadbalancer

import (
	"crypto/tls"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/opts"
	"github.com/docker/engine-api/client"
	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
	"net/http"
	"os"
	"runtime"
)

// NewDockerClient creates a new API client.
func NewDockerClient(host string, tls *tlsconfig.Options) (client.APIClient, error) {
	tlsOptions := tls
	host, err := getServerHost(host, tlsOptions)
	if err != nil {
		return &client.Client{}, err
	}

	customHeaders := map[string]string{
		"User-Agent": clientUserAgent(),
	}

	verStr := api.DefaultVersion
	if tmpStr := os.Getenv("DOCKER_API_VERSION"); tmpStr != "" {
		verStr = tmpStr
	}

	httpClient, err := newHTTPClient(host, tlsOptions)
	if err != nil {
		return &client.Client{}, err
	}

	return client.NewClient(host, verStr, httpClient, customHeaders)
}

func getServerHost(host string, tlsOptions *tlsconfig.Options) (string, error) {
	if host == "" {
		host = os.Getenv("DOCKER_HOST")
	}
	return opts.ParseHost(tlsOptions != nil, host)
}

func newHTTPClient(host string, tlsOptions *tlsconfig.Options) (*http.Client, error) {

	var config *tls.Config
	var err error

	if tlsOptions != nil {
		config, err = tlsconfig.Client(*tlsOptions)
		if err != nil {
			return nil, err
		}
		log.Infoln("TLS config=", config)
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
	return "Docker-Client/" + dockerversion.Version + " (" + runtime.GOOS + ")"
}

package hello

import (
	"crypto/tls"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
	"net/http"
	"os"
	"runtime"
	"time"
)

const (
	clientVersion = "1.24"
)

// NewDockerClient creates a new API client.
func NewDockerClient(host string, tls *tlsconfig.Options) (client.APIClient, error) {
	tlsOptions := tls
	if tls.KeyFile == "" || tls.CAFile == "" || tls.CertFile == "" {
		// The api doesn't like it when you pass in not nil but with zero field values...
		tlsOptions = nil
	}
	customHeaders := map[string]string{
		"User-Agent": clientUserAgent(),
	}
	verStr := clientVersion
	if tmpStr := os.Getenv("DOCKER_API_VERSION"); tmpStr != "" {
		verStr = tmpStr
	}
	httpClient, err := newHTTPClient(host, tlsOptions)
	if err != nil {
		return &client.Client{}, err
	}

	if cl, err := client.NewClient(host, verStr, httpClient, customHeaders); err == nil {
		return cl, err
	} else if err == client.ErrConnectionFailed {
		log.Infoln("Connection to docker failed.  Retrying")
		tick := time.Tick(1 * time.Second)
		deadline := time.After(10 * time.Minute)
	retry:
		for {
			select {
			case <-tick:
				log.Infoln("Retrying to connect to docker")
				if cl, err := client.NewClient(host, verStr, httpClient, customHeaders); err == nil {
					return cl, err
				}

			case <-deadline:
				break retry
			}
		}
	}
	log.Warningln("Deadline exceeded in connecting to docker")
	return nil, fmt.Errorf("deadline-exceeded")
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
	return fmt.Sprintf("Docker-Client/%s (%s)", clientVersion, runtime.GOOS)
}

package instance

import (
	"fmt"
	"log"

	"github.com/docker/docker/client"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/spf13/pflag"
)

const (
	defaultDockerURL        = "unix:///var/run/docker.sock"
	defaultDockerAPIVersion = "1.25"
)

type options struct {
	dockerURL        string
	dockerAPIVersion string
	//todo: add TLS options
	retries int
}

// Builder is a ProvisionerBuilder that creates a Docker instance provisioner
type Builder struct {
	client  *client.Client
	options options
}

// DockerClient returns the docker client
func (b *Builder) DockerClient() *client.Client {
	return b.client
}

// Flags returns the flags required.
func (b *Builder) Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("docker", pflag.PanicOnError)
	flags.StringVar(&b.options.dockerURL, "url", defaultDockerURL, "Docker API URL")
	flags.StringVar(&b.options.dockerAPIVersion, "version", defaultDockerAPIVersion, "Docker API version")
	flags.IntVar(&b.options.retries, "retries", 5, "Number of retries for Docker API operations")
	return flags
}

// BuildInstancePlugin creates an instance Provisioner configured with the Flags.
func (b *Builder) BuildInstancePlugin(namespaceTags map[string]string) (instance.Plugin, error) {
	if b.client == nil {
		defaultHeaders := map[string]string{"User-Agent": "InfraKit"}
		cli, err := client.NewClient(b.options.dockerURL, b.options.dockerAPIVersion, nil, defaultHeaders)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to Docker on [%s]\n%v", b.options.dockerURL, err)
		}
		b.client = cli
	}
	return NewInstancePlugin(b.client, namespaceTags), nil
}

type logger struct {
	logger *log.Logger
}

func (l logger) Log(args ...interface{}) {
	l.logger.Println(args...)
}

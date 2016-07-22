package loadbalancer

import (
	"github.com/docker/engine-api/types/swarm"
	mock_client "github.com/docker/libmachete/mock/docker/engine-api/client"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"testing"
	"time"
)

func TestBuildPoller(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock_client.NewMockAPIClient(ctrl)

	poller, err := NewServicePoller(client, 1*time.Second).Build()

	require.NoError(t, err)
	require.NotNil(t, poller)
}

func TestMatchMaps(t *testing.T) {
	require.True(t, matchMaps(
		map[string]string{"foo.bar": ""},
		map[string]string{"bar": "baz", "foo.bar": "bar"}))

	require.True(t, matchMaps(
		map[string]string{"foo.bar": "bar"},
		map[string]string{"bar": "baz", "foo.bar": "bar"}))

	require.False(t, matchMaps(
		map[string]string{"foo.bar": "baz"},
		map[string]string{"bar": "baz", "foo.bar": "bar"}))

	require.False(t, matchMaps(
		map[string]string{"foobar": "bar"},
		map[string]string{"bar": "baz", "foo.bar": "bar"}))

	require.True(t, matchMaps(
		map[string]string{"docker.editions.proxy.port": ""},
		map[string]string{"bar": "baz", "docker.editions.proxy.port": "80/http"}))
}

func TestDifferent(t *testing.T) {
	services := []swarm.Service{
		{
			Spec: swarm.ServiceSpec{
				Annotations: swarm.Annotations{
					Name: "proxy",
				},
			},
		},
		{
			Spec: swarm.ServiceSpec{
				Annotations: swarm.Annotations{
					Name: "service",
					Labels: map[string]string{
						"docker.editions.service.host": "service.foo.com",
					},
				},
			},
		},
	}

	require.False(t, different(services, services))

	updated := []swarm.Service{
		{
			Spec: swarm.ServiceSpec{
				Annotations: swarm.Annotations{
					Name: "proxy",
					Labels: map[string]string{
						"docker.editions.proxy.port": "80/http",
					},
				},
			},
		},
		{
			Spec: swarm.ServiceSpec{
				Annotations: swarm.Annotations{
					Name: "service",
					Labels: map[string]string{
						"docker.editions.service.host": "service.foo.com",
					},
				},
			},
		},
	}

	require.True(t, different(services, updated))

	updated = []swarm.Service{
		{
			Spec: swarm.ServiceSpec{
				Annotations: swarm.Annotations{
					Name: "proxy",
					Labels: map[string]string{
						"docker.editions.proxy.port": "80/http",
					},
				},
			},
			Endpoint: swarm.Endpoint{
				Ports: []swarm.PortConfig{
					{
						Protocol:      swarm.PortConfigProtocol("tcp"),
						TargetPort:    uint32(8080),
						PublishedPort: uint32(8080),
					},
				},
			},
		},
		{
			Spec: swarm.ServiceSpec{
				Annotations: swarm.Annotations{
					Name: "service",
					Labels: map[string]string{
						"docker.editions.service.host": "service.foo.com",
					},
				},
			},
		},
	}

	require.True(t, different(services, updated))
}

func TestRunPoller(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	services := []swarm.Service{
		{
			Spec: swarm.ServiceSpec{
				Annotations: swarm.Annotations{
					Name: "proxy",
					Labels: map[string]string{
						"docker.editions.proxy.port": "80/http",
					},
				},
			},
		},
		{
			Spec: swarm.ServiceSpec{
				Annotations: swarm.Annotations{
					Name: "service",
					Labels: map[string]string{
						"docker.editions.service.host": "service.foo.com",
					},
				},
			},
		},
	}

	client := mock_client.NewMockAPIClient(ctrl)
	client.EXPECT().ServiceList(gomock.Any(), gomock.Any()).AnyTimes().Return(services, nil)

	found := []swarm.Service{}

	stopPoller := make(chan interface{})

	poller, err := NewServicePoller(client, 200*time.Millisecond).
		AddService("proxy",

			MatchSpecLabels(map[string]string{
				"docker.editions.proxy.port": "",
			}),

			func(s []swarm.Service) {
				require.Equal(t, 1, len(s))
				require.Equal(t, "80/http", s[0].Spec.Labels["docker.editions.proxy.port"])
				found = append(found, s[0])

				close(stopPoller)
			}).
		Build()

	require.NoError(t, err)
	require.NotNil(t, poller)

	go func() {
		<-stopPoller
		poller.Stop()
	}()

	ctx := context.Background()
	poller.Run(ctx) // Blocks until stop() is called

	require.Equal(t, 1, len(found))
	require.Equal(t, services[0], found[0])
}

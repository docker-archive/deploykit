package swarm

import (
	"testing"

	"github.com/docker/docker/api/types/swarm"
	ingress "github.com/docker/infrakit/pkg/controller/ingress/types"
	mock_client "github.com/docker/infrakit/pkg/mock/docker/docker/client"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestBuildPoller(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock_client.NewMockAPIClientCloser(ctrl)

	routes, err := NewServiceRoutes(client).Build()

	require.NoError(t, err)
	require.NotNil(t, routes)
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

func TestRunRoutes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	certLabel := "certLabel"
	certID := "certID"
	healthLabel := "certLabel"
	healthPath := "healthPath"

	services := []swarm.Service{
		{
			Spec: swarm.ServiceSpec{
				Annotations: swarm.Annotations{
					Name: "proxy",
					Labels: map[string]string{
						"docker.editions.proxy.port": "80/http",
						certLabel:                    certID,
						healthLabel:                  healthPath,
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

	client := mock_client.NewMockAPIClientCloser(ctrl)
	client.EXPECT().ServiceList(gomock.Any(), gomock.Any()).AnyTimes().Return(services, nil)

	found := []swarm.Service{}

	stopRoutes := make(chan interface{})

	routes, err := NewServiceRoutes(client).
		SetCertLabel(&certLabel).
		SetHealthMonitorPathLabel(&healthLabel).
		AddRule("proxy",

			MatchSpecLabels(map[string]string{
				"docker.editions.proxy.port": "",
			}),

			func(s []swarm.Service) (map[ingress.Vhost][]loadbalancer.Route, error) {
				require.Equal(t, 1, len(s))
				require.Equal(t, "80/http", s[0].Spec.Labels["docker.editions.proxy.port"])
				found = append(found, s[0])

				close(stopRoutes)
				return nil, nil // returns no routes
			}).
		Build()

	require.NoError(t, err)
	require.NotNil(t, routes)

	vhostRoutes, err := routes.List()
	require.NoError(t, err)
	require.Equal(t, 0, len(vhostRoutes)) // because the matcher returns no routes

	require.Equal(t, 1, len(found))
	require.Equal(t, services[0], found[0])
}

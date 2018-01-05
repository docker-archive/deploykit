package ingress

import (
	"testing"

	"github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/stretchr/testify/require"
)

func TestPublishRoute(t *testing.T) {
	// Prime the lb with nothing
	lb := NewMockLBPlugin([]loadbalancer.Route{})

	// Construct desired list
	cert := "mycert"
	hmPath := "/health"
	desired := []loadbalancer.Route{{
		Port:                 8080,
		Protocol:             "HTTP",
		LoadBalancerPort:     444,
		LoadBalancerProtocol: "HTTPS",
		Certificate:          &cert,
		HealthMonitorPath:    &hmPath,
	}}

	var options types.Options
	err := configureL4(lb, desired, options)
	require.NoError(t, err)

	routes, err := lb.Routes()
	require.NoError(t, err)
	require.Equal(t, len(desired), len(routes))
	require.Equal(t, desired[0].Port, routes[0].Port)
	require.Equal(t, desired[0].Protocol, routes[0].Protocol)
	require.Equal(t, desired[0].LoadBalancerPort, routes[0].LoadBalancerPort)
	require.Equal(t, desired[0].LoadBalancerProtocol, routes[0].LoadBalancerProtocol)
	require.Equal(t, *desired[0].Certificate, *routes[0].Certificate)
	require.Equal(t, *desired[0].HealthMonitorPath, *routes[0].HealthMonitorPath)
}

func TestUnpublishRoute(t *testing.T) {
	// Prime the lb with a route
	cert := "mycert"
	hmPath := "/health"
	lb := NewMockLBPlugin([]loadbalancer.Route{{
		Port:                 8080,
		Protocol:             "HTTP",
		LoadBalancerPort:     444,
		LoadBalancerProtocol: "HTTPS",
		Certificate:          &cert,
		HealthMonitorPath:    &hmPath,
	}})

	// Construct empty desired list
	desired := []loadbalancer.Route{}

	var options types.Options
	err := configureL4(lb, desired, options)
	require.NoError(t, err)

	routes, err := lb.Routes()
	require.NoError(t, err)
	require.Equal(t, len(desired), len(routes))
}

func TestChangeCert(t *testing.T) {
	// Prime the lb with a route
	cert := "mycert"
	hmPath := "/health"
	lb := NewMockLBPlugin([]loadbalancer.Route{{
		Port:                 8080,
		Protocol:             "HTTP",
		LoadBalancerPort:     444,
		LoadBalancerProtocol: "HTTPS",
		Certificate:          &cert,
		HealthMonitorPath:    &hmPath,
	}})

	// Change the cert
	newcert := "newcert"
	desired := []loadbalancer.Route{{
		Port:                 8080,
		Protocol:             "HTTP",
		LoadBalancerPort:     444,
		LoadBalancerProtocol: "HTTPS",
		Certificate:          &newcert,
		HealthMonitorPath:    &hmPath,
	}}

	var options types.Options
	err := configureL4(lb, desired, options)
	require.NoError(t, err)

	routes, err := lb.Routes()
	require.NoError(t, err)
	require.Equal(t, len(desired), len(routes))
	require.Equal(t, desired[0].Port, routes[0].Port)
	require.Equal(t, desired[0].Protocol, routes[0].Protocol)
	require.Equal(t, desired[0].LoadBalancerPort, routes[0].LoadBalancerPort)
	require.Equal(t, desired[0].LoadBalancerProtocol, routes[0].LoadBalancerProtocol)
	require.Equal(t, *desired[0].Certificate, *routes[0].Certificate)
	require.Equal(t, *desired[0].HealthMonitorPath, *routes[0].HealthMonitorPath)
}

func TestChangeHmPath(t *testing.T) {
	// Prime the lb with a route
	cert := "mycert"
	hmPath := "/health"
	lb := NewMockLBPlugin([]loadbalancer.Route{{
		Port:                 8080,
		Protocol:             "HTTP",
		LoadBalancerPort:     444,
		LoadBalancerProtocol: "HTTPS",
		Certificate:          &cert,
		HealthMonitorPath:    &hmPath,
	}})

	// Change the health monitor path
	newHmPath := "/newpath"
	desired := []loadbalancer.Route{{
		Port:                 8080,
		Protocol:             "HTTP",
		LoadBalancerPort:     444,
		LoadBalancerProtocol: "HTTPS",
		Certificate:          &cert,
		HealthMonitorPath:    &newHmPath,
	}}

	var options types.Options
	err := configureL4(lb, desired, options)
	require.NoError(t, err)

	routes, err := lb.Routes()
	require.NoError(t, err)
	require.Equal(t, len(desired), len(routes))
	require.Equal(t, desired[0].Port, routes[0].Port)
	require.Equal(t, desired[0].Protocol, routes[0].Protocol)
	require.Equal(t, desired[0].LoadBalancerPort, routes[0].LoadBalancerPort)
	require.Equal(t, desired[0].LoadBalancerProtocol, routes[0].LoadBalancerProtocol)
	require.Equal(t, *desired[0].Certificate, *routes[0].Certificate)
	require.Equal(t, *desired[0].HealthMonitorPath, *routes[0].HealthMonitorPath)
}

func TestVarietyOperations(t *testing.T) {
	// Have a changed route, an unchanged route, a new route and a route to be deleted
	cert := "mycert"
	hmPath := "/health"
	lb := NewMockLBPlugin([]loadbalancer.Route{{
		// This one will change
		Port:                 8080,
		Protocol:             "HTTP",
		LoadBalancerPort:     444,
		LoadBalancerProtocol: "HTTPS",
		Certificate:          &cert,
		HealthMonitorPath:    &hmPath,
	}, {
		// This one will match
		Port:                 8081,
		Protocol:             "HTTP",
		LoadBalancerPort:     445,
		LoadBalancerProtocol: "HTTPS",
		Certificate:          &cert,
		HealthMonitorPath:    &hmPath,
	}, {
		// This one is unmatched (should be removed)
		Port:                 8081,
		Protocol:             "HTTP",
		LoadBalancerPort:     446,
		LoadBalancerProtocol: "HTTPS",
		Certificate:          &cert,
		HealthMonitorPath:    &hmPath,
	}})

	// Change the cert
	newcert := "newcert"
	desired := []loadbalancer.Route{{
		Port:                 8080,
		Protocol:             "HTTP",
		LoadBalancerPort:     444,
		LoadBalancerProtocol: "HTTPS",
		Certificate:          &newcert,
		HealthMonitorPath:    &hmPath,
	}, {
		Port:                 8081,
		Protocol:             "HTTP",
		LoadBalancerPort:     445,
		LoadBalancerProtocol: "HTTPS",
		Certificate:          &cert,
		HealthMonitorPath:    &hmPath,
	}, {
		// This one should be added
		Port:                 8081,
		Protocol:             "HTTP",
		LoadBalancerPort:     447,
		LoadBalancerProtocol: "HTTPS",
		Certificate:          &cert,
		HealthMonitorPath:    &hmPath,
	}}

	var options types.Options
	err := configureL4(lb, desired, options)
	require.NoError(t, err)

	routes, err := lb.Routes()
	require.NoError(t, err)
	require.Equal(t, len(desired), len(routes))
	for _, route := range routes {
		found := false
		for _, expected := range desired {
			if expected.LoadBalancerPort == route.LoadBalancerPort {
				require.Equal(t, expected.Port, route.Port)
				require.Equal(t, expected.Protocol, route.Protocol)
				require.Equal(t, expected.LoadBalancerPort, route.LoadBalancerPort)
				require.Equal(t, expected.LoadBalancerProtocol, route.LoadBalancerProtocol)
				require.Equal(t, *expected.Certificate, *route.Certificate)
				require.Equal(t, *expected.HealthMonitorPath, *route.HealthMonitorPath)
				found = true
				break
			} else {
				continue
			}
		}
		if !found {
			require.Fail(t, "Match was not found")
		}
	}
}

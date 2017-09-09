package loadbalancer

import (
	"errors"
	"io/ioutil"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/plugin"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	testing_lb "github.com/docker/infrakit/pkg/testing/loadbalancer"
	"github.com/stretchr/testify/require"
)

type fakeResult string

func (f fakeResult) String() string {
	return string(f)
}

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return path.Join(dir, "loadbalancer-impl-test")
}

func must(l4 loadbalancer.L4, err error) loadbalancer.L4 {
	if err != nil {
		panic(err)
	}
	return l4
}

func TestLoadbalancerName(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	expectedName := "some-name"

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_lb.L4{
		DoName: func() string {
			return expectedName
		},
	}))
	require.NoError(t, err)

	actualName := must(NewClient(plugin.Name(name+"/type1"), socketPath)).Name()
	server.Stop()
	require.Equal(t, expectedName, actualName)
}

func TestLoadbalancerRoutes(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	routesActual := make(chan []loadbalancer.Route, 1)
	routes := []loadbalancer.Route{
		{
			Port:                 1234,
			Protocol:             loadbalancer.HTTP,
			LoadBalancerPort:     2345,
			LoadBalancerProtocol: loadbalancer.HTTP,
		},
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_lb.L4{
		DoRoutes: func() ([]loadbalancer.Route, error) {
			routesActual <- routes
			return routes, nil
		},
	}))
	require.NoError(t, err)

	result, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).Routes()
	server.Stop()
	require.NoError(t, err)
	require.Equal(t, routes, result)
}

func TestLoadbalancerRoutesError(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_lb.L4{
		DoRoutes: func() ([]loadbalancer.Route, error) {
			return nil, errors.New("nope")
		},
	}))
	require.NoError(t, err)

	result, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).Routes()
	server.Stop()
	require.Error(t, err)
	require.Equal(t, "nope", err.Error())
	require.Nil(t, result)
}

func TestLoadbalancerPublish(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	routeActual := make(chan loadbalancer.Route, 1)
	route := loadbalancer.Route{
		Port:                 1234,
		Protocol:             loadbalancer.HTTP,
		LoadBalancerPort:     2345,
		LoadBalancerProtocol: loadbalancer.HTTP,
	}
	result := fakeResult("result")

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_lb.L4{
		DoPublish: func(route loadbalancer.Route) (loadbalancer.Result, error) {
			routeActual <- route
			return result, nil
		},
	}))
	require.NoError(t, err)

	actualResult, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).Publish(route)
	server.Stop()
	require.NoError(t, err)
	require.Equal(t, result.String(), actualResult.String())
	require.Equal(t, route, <-routeActual)
}

func TestLoadbalancerPublishError(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	routeActual := make(chan loadbalancer.Route, 1)
	route := loadbalancer.Route{
		Port:                 1234,
		Protocol:             loadbalancer.HTTP,
		LoadBalancerPort:     2345,
		LoadBalancerProtocol: loadbalancer.HTTP,
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_lb.L4{
		DoPublish: func(route loadbalancer.Route) (loadbalancer.Result, error) {
			routeActual <- route
			return nil, errors.New("backend-error")
		},
	}))
	require.NoError(t, err)

	result, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).Publish(route)
	server.Stop()
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, route, <-routeActual)
}

func TestLoadbalancerUnpublish(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	portActual := make(chan int, 1)
	port := 1234

	result := fakeResult("result")

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_lb.L4{
		DoUnpublish: func(port int) (loadbalancer.Result, error) {
			portActual <- port
			return result, nil
		},
	}))
	require.NoError(t, err)

	actualResult, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).Unpublish(port)
	server.Stop()
	require.NoError(t, err)
	require.Equal(t, result.String(), actualResult.String())
	require.Equal(t, port, <-portActual)
}

func TestLoadbalancerUnpublishError(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	portActual := make(chan int, 1)
	port := 1234

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_lb.L4{
		DoUnpublish: func(port int) (loadbalancer.Result, error) {
			portActual <- port
			return nil, errors.New("backend-error")
		},
	}))
	require.NoError(t, err)

	result, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).Unpublish(port)
	server.Stop()
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, port, <-portActual)
}

func TestLoadbalancerConfigureHealthCheck(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	portActual := make(chan int, 1)
	port := 1234
	healthyActual := make(chan int, 1)
	healthy := 1
	unhealthyActual := make(chan int, 1)
	unhealthy := 2
	intervalActual := make(chan time.Duration, 1)
	interval := time.Duration(time.Second * 20)
	timeoutActual := make(chan time.Duration, 1)
	timeout := time.Duration(time.Minute * 2)

	result := fakeResult("result")

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_lb.L4{
		DoConfigureHealthCheck: func(hc loadbalancer.HealthCheck) (loadbalancer.Result, error) {
			portActual <- hc.BackendPort
			healthyActual <- hc.Healthy
			unhealthyActual <- hc.Unhealthy
			intervalActual <- hc.Interval
			timeoutActual <- hc.Timeout
			return result, nil
		},
	}))
	require.NoError(t, err)

	actualResult, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).ConfigureHealthCheck(
		loadbalancer.HealthCheck{
			BackendPort: port,
			Healthy:     healthy,
			Unhealthy:   unhealthy,
			Interval:    interval,
			Timeout:     timeout,
		})
	server.Stop()
	require.NoError(t, err)
	require.Equal(t, result.String(), actualResult.String())
	require.Equal(t, port, <-portActual)
	require.Equal(t, healthy, <-healthyActual)
	require.Equal(t, unhealthy, <-unhealthyActual)
	require.Equal(t, interval, <-intervalActual)
	require.Equal(t, timeout, <-timeoutActual)
}

func TestLoadbalancerConfigureHealthCheckError(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	portActual := make(chan int, 1)
	port := 1234
	healthyActual := make(chan int, 1)
	healthy := 1
	unhealthyActual := make(chan int, 1)
	unhealthy := 2
	intervalActual := make(chan time.Duration, 1)
	interval := time.Duration(time.Second * 20)
	timeoutActual := make(chan time.Duration, 1)
	timeout := time.Duration(time.Minute * 2)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_lb.L4{
		DoConfigureHealthCheck: func(hc loadbalancer.HealthCheck) (loadbalancer.Result, error) {
			portActual <- hc.BackendPort
			healthyActual <- hc.Healthy
			unhealthyActual <- hc.Unhealthy
			intervalActual <- hc.Interval
			timeoutActual <- hc.Timeout
			return nil, errors.New("backend-error")
		},
	}))
	require.NoError(t, err)

	result, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).ConfigureHealthCheck(
		loadbalancer.HealthCheck{
			BackendPort: port,
			Healthy:     healthy,
			Unhealthy:   unhealthy,
			Interval:    interval,
			Timeout:     timeout,
		})
	server.Stop()
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, port, <-portActual)
	require.Equal(t, healthy, <-healthyActual)
	require.Equal(t, unhealthy, <-unhealthyActual)
	require.Equal(t, interval, <-intervalActual)
	require.Equal(t, timeout, <-timeoutActual)
}

func TestLoadbalancerRegisterBackend(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	actual := make(chan []instance.ID, 1)
	expected := []instance.ID{instance.ID("some-id"), instance.ID("some"), instance.ID("more"), instance.ID("data")}
	result := fakeResult("result")

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_lb.L4{
		DoRegisterBackends: func(ids []instance.ID) (loadbalancer.Result, error) {
			actual <- ids
			return result, nil
		},
	}))
	require.NoError(t, err)

	actualResult, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).RegisterBackends(expected)
	server.Stop()
	require.NoError(t, err)
	require.Equal(t, result.String(), actualResult.String())
	require.Equal(t, expected, <-actual)
}

func TestLoadbalancerRegisterBackendError(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	actual := make(chan []instance.ID, 1)
	expected := []instance.ID{instance.ID("some-id"), instance.ID("some"), instance.ID("more"), instance.ID("data")}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_lb.L4{
		DoRegisterBackends: func(ids []instance.ID) (loadbalancer.Result, error) {
			actual <- ids
			return nil, errors.New("backend-error")
		},
	}))
	require.NoError(t, err)

	result, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).RegisterBackends(expected)
	server.Stop()
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, expected, <-actual)
}

func TestLoadbalancerDeregisterBackend(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	actual := make(chan []instance.ID, 1)
	expected := []instance.ID{instance.ID("some-id"), instance.ID("some"), instance.ID("more"), instance.ID("data")}
	result := fakeResult("result")

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_lb.L4{
		DoDeregisterBackends: func(ids []instance.ID) (loadbalancer.Result, error) {
			actual <- ids
			return result, nil
		},
	}))
	require.NoError(t, err)

	actualResult, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).DeregisterBackends(expected)
	server.Stop()
	require.NoError(t, err)
	require.Equal(t, result.String(), actualResult.String())
	require.Equal(t, expected, <-actual)
}

func TestLoadbalancerDeregisterBackendError(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	actual := make(chan []instance.ID, 1)
	expected := []instance.ID{instance.ID("some-id"), instance.ID("some"), instance.ID("more"), instance.ID("data")}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_lb.L4{
		DoDeregisterBackends: func(ids []instance.ID) (loadbalancer.Result, error) {
			actual <- expected
			return nil, errors.New("backend-error")
		},
	}))
	require.NoError(t, err)

	result, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).DeregisterBackends(expected)
	server.Stop()
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, expected, <-actual)
}

func TestLoadbalancerBackends(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	expected := []instance.ID{instance.ID("some-id"), instance.ID("some"), instance.ID("more"), instance.ID("data")}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_lb.L4{
		DoBackends: func() ([]instance.ID, error) {
			return expected, nil
		},
	}))
	require.NoError(t, err)

	actualResult, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).Backends()
	server.Stop()
	require.NoError(t, err)
	require.Equal(t, expected, actualResult)
}

func TestLoadbalancerBackendsError(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_lb.L4{
		DoBackends: func() ([]instance.ID, error) {
			return nil, errors.New("backend-error")
		},
	}))
	require.NoError(t, err)

	result, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).Backends()
	server.Stop()
	require.Error(t, err)
	require.Nil(t, result)
}

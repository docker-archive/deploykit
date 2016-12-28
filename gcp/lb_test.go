package gcp

import (
	"fmt"
	"testing"

	"github.com/docker/editions/pkg/loadbalancer"
	"github.com/stretchr/testify/require"
	compute "google.golang.org/api/compute/v1"
)

func TestClosePort(t *testing.T) {
	var tests = []struct {
		label           string
		allowed         []*compute.FirewallAllowed
		portToRemove    uint32
		expectedAllowed []*compute.FirewallAllowed
		expectedFound   bool
		expectedErr     error
	}{
		{"Not found", []*compute.FirewallAllowed{}, 9000, []*compute.FirewallAllowed{}, false, nil},
		{"Last port", []*compute.FirewallAllowed{{Ports: []string{"8080"}}}, 8080, []*compute.FirewallAllowed{{Ports: []string{}}}, true, nil},
		{"One port", []*compute.FirewallAllowed{{Ports: []string{"80", "8080"}}}, 80, []*compute.FirewallAllowed{{Ports: []string{"8080"}}}, true, nil},
		{"All instances of one port", []*compute.FirewallAllowed{{Ports: []string{"80", "8080"}}, {Ports: []string{"80", "9000"}}}, 80, []*compute.FirewallAllowed{{Ports: []string{"8080"}}, {Ports: []string{"9000"}}}, true, nil},
		{"Invalid port", []*compute.FirewallAllowed{{Ports: []string{"invalid"}}}, 80, []*compute.FirewallAllowed{{Ports: []string{"invalid"}}}, false, fmt.Errorf("Invalid port: %s", "invalid")},
	}

	for _, test := range tests {
		t.Log(test.label)

		firewall := &compute.Firewall{
			Allowed:test.allowed,
		}

		found, err := ClosePort(firewall, test.portToRemove)

		require.Equal(t, test.expectedFound, found)
		require.Equal(t, test.expectedErr, err)
		require.Equal(t, test.expectedAllowed, firewall.Allowed)
	}
}

func TestOpenPort(t *testing.T) {
	var tests = []struct {
		label           string
		allowed         []*compute.FirewallAllowed
		portToRemove    uint32
		protocol        loadbalancer.Protocol
		expectedAllowed []*compute.FirewallAllowed
		expectedFound   bool
		expectedErr     error
	}{
		{"First opened port", []*compute.FirewallAllowed{{IPProtocol:"tcp"}}, 9000, loadbalancer.TCP, []*compute.FirewallAllowed{{IPProtocol:"tcp", Ports: []string{"9000"}}}, false, nil},
		{"Add port", []*compute.FirewallAllowed{{IPProtocol:"tcp", Ports: []string{"9000"}}}, 80, loadbalancer.TCP, []*compute.FirewallAllowed{{IPProtocol:"tcp", Ports: []string{"9000", "80"}}}, false, nil},
		{"Already opened", []*compute.FirewallAllowed{{IPProtocol:"tcp", Ports: []string{"9000"}}}, 9000, loadbalancer.TCP, []*compute.FirewallAllowed{{IPProtocol:"tcp", Ports: []string{"9000"}}}, true, nil},
		{"Different protocol", []*compute.FirewallAllowed{{IPProtocol:"udp", Ports: []string{"9000"}}, {IPProtocol:"tcp", Ports: []string{"8080"}}}, 80, loadbalancer.TCP, []*compute.FirewallAllowed{{IPProtocol:"udp", Ports: []string{"9000"}}, {IPProtocol:"tcp", Ports: []string{"8080", "80"}}}, false, nil},
		{"First opened port for protocol", []*compute.FirewallAllowed{{IPProtocol:"udp", Ports: []string{"9000"}}}, 80, loadbalancer.TCP, []*compute.FirewallAllowed{{IPProtocol:"udp", Ports: []string{"9000"}}, {IPProtocol:"TCP", Ports: []string{"80"}}}, false, nil},
		{"Invalid port", []*compute.FirewallAllowed{{Ports: []string{"wrong"}}}, 80, loadbalancer.TCP, []*compute.FirewallAllowed{{Ports: []string{"wrong"}}}, false, fmt.Errorf("Invalid port: %s", "wrong")},
	}

	for _, test := range tests {
		t.Log(test.label)

		firewall := &compute.Firewall{
			Allowed:test.allowed,
		}

		found, err := OpenPort(firewall, test.portToRemove, test.protocol)

		require.Equal(t, test.expectedFound, found)
		require.Equal(t, test.expectedErr, err)
		require.Equal(t, test.expectedAllowed, firewall.Allowed)
	}
}

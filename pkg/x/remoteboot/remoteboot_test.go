package remoteboot

import (
	"testing"
)

func TestConvertIP(t *testing.T) {
	fixedIP := convertIP("10.0.0.1")
	if len(fixedIP) != 4 {
		t.Fatalf("Not converted IP address to a four byte sequence")
	}
}

func TestNewController(t *testing.T) {
	bootoutput, err := NewRemoteBoot("NICA",
		"192.168.0.1",
		"", //blank HTTP
		"", //blank TFTP
		"filename",
		"", //blank DNS
		"", //blank GATEWAY
		20,
		"192.168.0.2")
	if bootoutput.httpAddress != bootoutput.dhcpAddress {
		t.Fatalf("Error allocating HTTP address from DHCP address")
	}
	if bootoutput.tftpAddress != bootoutput.dhcpAddress {
		t.Fatalf("Error allocating TFTP address from DHCP address")
	}
	if bootoutput.handler.ip.String() != bootoutput.dhcpAddress {
		t.Fatalf("Error allocating DHCP lease address from DHCP address")
	}
	if err != nil {
		t.Fatalf("%v", err)
	}
}

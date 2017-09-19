package remoteboot

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	dhcp "github.com/krolaw/dhcp4"
	conn "github.com/krolaw/dhcp4/conn"
	tftp "github.com/thebsdbox/go-tftp/server"
)

// BootController contains the settings that define how the remote boot will
// behave
type BootController struct {
	adapterName string // A physical adapter to bind to e.g. en0, eth0

	// Servers
	dhcpServer  bool   // Enable Server
	dhcpAddress string // Should ideally be the IP of the adapter

	tftpServer  bool   // Enable Server
	tftpAddress string // Should ideally be the IP of the adapter

	httpServer  bool   // Enable Server
	httpAddress string // Should ideally be the IP of the adapter

	// TFTP Configuration
	pxeFileName string // undionly.kpxe

	// iPXE file settings - exported
	Kernel  string
	Initrd  string
	Cmdline string

	handler *DHCPSettings
}

// store the tftp files in ram
var iPXEData []byte

type lease struct {
	nic    string    // Client's Addr
	expiry time.Time // When the lease expires
}

// DHCPSettings -
type DHCPSettings struct {
	ip            net.IP        // Server IP to use
	options       dhcp.Options  // Options to send to DHCP Clients
	start         net.IP        // Start of IP range to distribute
	leaseRange    int           // Number of IPs to distribute (starting from start)
	leaseDuration time.Duration // Lease period
	leases        map[int]lease // Map to keep track of leases
}

// NewRemoteBoot -
func NewRemoteBoot(adapterName string,
	addressDHCP string,
	addressHTTP string,
	addressTFTP string,
	pxeFileName string,
	gateway string,
	dns string,
	leasecount int,
	startAddress string) (*BootController, error) {

	handler := &DHCPSettings{}
	if addressDHCP == "" {
		// No address is specified so find the one bound to the interface
		var err error
		addressDHCP, err = findIPAddress(adapterName)
		if err != nil {
			return nil, err
		}
	}

	// Set appropriate addresses for TFTP server
	if addressTFTP == "" {
		addressTFTP = addressDHCP
	}

	// Set appropriate addresses for HTTP server
	if addressHTTP == "" {
		addressHTTP = addressDHCP
	}

	log.Printf("Binding to %s / %s", adapterName, addressDHCP)

	handler.ip = convertIP(addressDHCP)

	// If not set then use the main address for these details
	if gateway == "" {
		gateway = addressDHCP
	}
	if dns == "" {
		dns = addressDHCP
	}

	handler.start = convertIP(startAddress)

	handler.leaseDuration = 2 * time.Hour //TODO, make time modifiable
	handler.leaseRange = leasecount
	handler.leases = make(map[int]lease, leasecount)

	handler.options = dhcp.Options{
		dhcp.OptionSubnetMask:       []byte{255, 255, 255, 0},
		dhcp.OptionRouter:           []byte(convertIP(gateway)),
		dhcp.OptionDomainNameServer: []byte(convertIP(dns)),
		dhcp.OptionBootFileName:     []byte(pxeFileName),
	}

	return &BootController{
		adapterName: adapterName,
		dhcpAddress: addressDHCP,
		tftpAddress: addressTFTP,
		httpAddress: addressHTTP,
		pxeFileName: pxeFileName,
		handler:     handler,
	}, nil
}

// StartServices - This will start all of the enabled services
func (c *BootController) StartServices(dhcpService bool, tftpService bool, httpService bool) {
	log.Println("Starting Remote Boot Services, press CTRL + c to stop")

	if dhcpService == true {
		go func() {
			log.Println("RemoteBoot => Starting DHCP")
			newConnection, err := conn.NewUDP4FilterListener(c.adapterName, ":67")
			if err != nil {
				log.Fatalf("%v", err)
			}
			err = dhcp.Serve(newConnection, c.handler)
			log.Fatalf("%v", err)
		}()
	}

	if tftpService == true {
		go func() {
			log.Println("RemoteBoot => Starting TFTP")
			err := c.serveTFTP()
			log.Fatalf("%v", err)
		}()
	}

	if httpService == true {
		go func() {
			log.Println("RemoteBoot => Starting HTTP")
			err := c.serveHTTP()
			log.Fatalf("%v", err)
		}()
	}
	waitForCtrlC()
}

//////////////////////////////
//
// DHCP Server
//
//////////////////////////////

//ServeDHCP -
func (h *DHCPSettings) ServeDHCP(p dhcp.Packet, msgType dhcp.MessageType, options dhcp.Options) (d dhcp.Packet) {
	//log.Infof("DCHP Message: %v", msgType)
	switch msgType {

	case dhcp.Discover:
		if string(options[77]) != "" {
			if string(options[77]) == "iPXE" {
				h.options[67] = []byte("http://" + h.ip.String() + "/infrakit.ipxe")
			}
		}
		free, nic := -1, p.CHAddr().String()
		for i, v := range h.leases { // Find previous lease
			if v.nic == nic {
				free = i
				goto reply
			}
		}
		if free = h.freeLease(); free == -1 {
			return
		}
	reply:
		h.options[60] = h.ip
		ipLease := dhcp.IPAdd(h.start, free)

		return dhcp.ReplyPacket(p, dhcp.Offer, h.ip, ipLease, h.leaseDuration,
			h.options.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]))

	case dhcp.Request:
		if server, ok := options[dhcp.OptionServerIdentifier]; ok && !net.IP(server).Equal(h.ip) {
			return nil // Message not for this dhcp server
		}
		reqIP := net.IP(options[dhcp.OptionRequestedIPAddress])
		if reqIP == nil {
			reqIP = net.IP(p.CIAddr())
		}

		if len(reqIP) == 4 && !reqIP.Equal(net.IPv4zero) {
			if leaseNum := dhcp.IPRange(h.start, reqIP) - 1; leaseNum >= 0 && leaseNum < h.leaseRange {
				if l, exists := h.leases[leaseNum]; !exists || l.nic == p.CHAddr().String() {
					h.leases[leaseNum] = lease{nic: p.CHAddr().String(), expiry: time.Now().Add(h.leaseDuration)}
					return dhcp.ReplyPacket(p, dhcp.ACK, h.ip, reqIP, h.leaseDuration,
						h.options.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]))
				}
			}
		}
		return dhcp.ReplyPacket(p, dhcp.NAK, h.ip, nil, 0, nil)

	case dhcp.Release, dhcp.Decline:
		nic := p.CHAddr().String()
		for i, v := range h.leases {
			if v.nic == nic {
				delete(h.leases, i)
				break
			}
		}
	}
	return nil
}

func (h *DHCPSettings) freeLease() int {
	now := time.Now()
	b := rand.Intn(h.leaseRange) // Try random first
	for _, v := range [][]int{{b, h.leaseRange}, {0, b}} {
		for i := v[0]; i < v[1]; i++ {
			if l, ok := h.leases[i]; !ok || l.expiry.Before(now) {
				return i
			}
		}
	}
	return -1
}

//////////////////////////////
//
// TFTP Server
//
//////////////////////////////

// HandleWrite : writing is disabled in this service
func HandleWrite(filename string) (w io.Writer, err error) {
	err = errors.New("Server is read only")
	return
}

// HandleRead : read a ROfs file and send over tftp
func HandleRead(filename string) (r io.Reader, err error) {
	r = bytes.NewBuffer(iPXEData)
	return
}

// tftp server
func (c *BootController) serveTFTP() error {

	log.Printf("Opening and caching undionly.kpxe")
	f, err := os.Open(c.pxeFileName)
	if err != nil {
		log.Printf("Please download the bootloader with the pulliPXE command")
		return err
	}
	// Use bufio.NewReader to get a Reader.
	// ... Then use ioutil.ReadAll to read the entire content.
	r := bufio.NewReader(f)

	iPXEData, err = ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	s := tftp.NewServer("", HandleRead, HandleWrite)
	err = s.Serve(c.tftpAddress + ":69")
	if err != nil {
		return err
	}
	return nil
}

//////////////////////////////
//
// HTTP Server
//
//////////////////////////////

func (c *BootController) serveHTTP() error {
	if _, err := os.Stat("./infrakit.ipxe"); os.IsNotExist(err) {
		log.Println("Auto generating ./infrakit.ipxe")
		err = generateiPXEScript(c.httpAddress, c.Kernel, c.Initrd, c.Cmdline)
		if err != nil {
			return err
		}
	}

	docroot, err := filepath.Abs("./")
	if err != nil {
		return err
	}

	httpHandler := http.FileServer(http.Dir(docroot))

	return http.ListenAndServe(":80", httpHandler)
}

//////////////////////////////
//
// Helper Functions
//
//////////////////////////////

func generateiPXEScript(webserverAddress string, kernel string, initrd string, cmdline string) error {
	script := `#!ipxe

dhcp
echo +-----------  Infrakit Remote Boot  -------------------------
echo | hostname: ${hostname}, next-server: ${next-server}
echo | address.: ${net0/ip}
echo | mac.....: ${net0/mac}  
echo | gateway.: ${net0/gateway} 
echo +------------------------------------------------------------
echo .
kernel http://%s/%s %s 
initrd http://%s/%s
boot`
	// Replace the addresses inline
	buildScript := fmt.Sprintf(script, webserverAddress, kernel, cmdline, webserverAddress, initrd)

	f, err := os.Create("./infrakit.ipxe")
	if err != nil {
		return err
	}
	_, err = f.WriteString(buildScript)
	if err != nil {
		return err
	}
	f.Sync()
	return nil
}

func waitForCtrlC() {
	var endWaiter sync.WaitGroup
	endWaiter.Add(1)
	var signalChannel chan os.Signal
	signalChannel = make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)
	go func() {
		<-signalChannel
		endWaiter.Done()
	}()
	endWaiter.Wait()
}

func findIPAddress(addrName string) (string, error) {
	var address string
	list, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range list {
		if iface.Name == addrName {
			addrs, err := iface.Addrs()
			if err != nil {
				return "", err
			}
			for _, a := range addrs {
				if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						address = ipnet.IP.String()
					}
				}
			}
		}
	}
	return address, nil
}

func convertIP(ipAddress string) []byte {
	// net.ParseIP has returned IPv6 sized allocations o_O
	fixIP := net.ParseIP(ipAddress)
	if fixIP == nil {
		log.Fatalf("Couldn't parse the IP address: %s\n", ipAddress)
	}
	if len(fixIP) > 4 {
		return fixIP[len(fixIP)-4:]
	}
	return fixIP
}

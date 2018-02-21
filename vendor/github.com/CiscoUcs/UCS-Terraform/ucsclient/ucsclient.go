package ucsclient

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	ucs "github.com/CiscoUcs/UCS-Terraform/ucsclient/ucsinternal"
	"github.com/micdoher/GoUtils"
	xmlpath "gopkg.in/xmlpath.v2"
)

const BODY_TYPE_XML = "text/xml"

type (
	HTTPClient interface {
		Post(string, string, io.Reader) (*http.Response, error)
	}

	ServiceProfile struct {
		Name         string
		Template     string
		TargetOrg    string
		Hierarchical bool
		VNICs        []VNIC
	}

	UCSClient struct {
		httpClient            HTTPClient
		ipAddress             string
		username              string
		password              string
		tslInsecureSkipVerify bool
		cookie                string
		outDomains            string
		appName               string
		Logger                *utils.Logger
	}

	VNIC struct {
		Name string
		Mac  string
		CIDR string
		Ip   net.IP
	}
)

// The XML request sent to the UCS server to create a Service Profile *must* include
// a cookie which does not belong to the ServiceProfile model per se, hence the
// `cookie` argument required.
func (sp *ServiceProfile) Marshal(cookie string) ([]byte, error) {
	template := strings.Join([]string{sp.TargetOrg, "/", "ls-", sp.Template}, "")
	spr := ucs.ServiceProfileRequest{
		Cookie:          cookie,
		Dn:              template,
		TargetOrg:       sp.TargetOrg,
		ErrorOnExisting: true,
		InNameSet: ucs.InNameSet{
			Dn: ucs.Dn{
				Value: sp.Name,
			},
		},
	}
	return xml.Marshal(spr)
}

func (sp *ServiceProfile) ToJSON() (string, error) {
	b, err := json.Marshal(sp)
	if err != nil {
		return "", nil
	}

	return string(b), nil
}

func (sp *ServiceProfile) DN() string {
	return sp.TargetOrg + "/ls-" + sp.Name
}

func NewUCSClient(c *Config) *UCSClient {
	client := UCSClient{
		ipAddress:             c.IpAddress,
		username:              c.Username,
		password:              c.Password,
		tslInsecureSkipVerify: c.TslInsecureSkipVerify,
		appName:               c.AppName,
	}

	client.httpClient = NewHTTPClient(c.TslInsecureSkipVerify)
	client.Logger = utils.NewLogger(getLogFile(c.LogFilename), c.LogLevel)
	client.Logger.Print("Log level %v\n", c.LogLevel)

	return &client
}

func NewHTTPClient(insecureSkipVerify bool) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify},
		},
	}
}

func (c *UCSClient) Destroy(name, targetOrg string, hierarchical bool) error {
	req := ucs.DestroyRequest{
		Name:         name,
		TargetOrg:    targetOrg,
		Hierarchical: hierarchical,
	}
	payload, err := req.Marshal(c.cookie)
	if err != nil {
		return err
	}

	_, err = c.Post(payload)
	if err != nil {
		return err
	}

	return nil
}

// Performs a POST request to the UCS Server.
// Returns a string with the response's body.
// In case that an error happens it will return an empty string
// along with an error.
func (c *UCSClient) Post(payload []byte) ([]byte, error) {
	c.Logger.Debug("POST %s\n", c.endpointURL())
	c.Logger.Debug("Payload: %s\n", payload)

	res, err := c.httpClient.Post(c.endpointURL(), BODY_TYPE_XML, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	c.Logger.Debug("Message successfully sent\n")

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	c.Logger.Debug("Response: %s\n", body)

	return body, nil
}

// Logs into the UCS server by posting the username and password in
// the config.
// Returns an error if anything goes wrong.
func (c *UCSClient) Login() error {
	req := ucs.LoginRequest{
		Username: c.username,
		Password: c.password,
	}
	payload, err := req.Marshal()
	if err != nil {
		return err
	}

	data, err := c.Post(payload)
	if err != nil {
		return err
	}

	res, err := ucs.NewLoginResponse(data)
	if err != nil {
		return err
	}

	c.cookie = res.OutCookie
	c.outDomains = res.OutDomains
	c.Logger.Debug("Successfully logged in with cookie %s \n", c.cookie)
	return nil
}

// Logs out of the UCS server.
func (c *UCSClient) Logout() {
	if c.IsLoggedIn() {
		c.Logger.Debug("Logging out\n")
		req := ucs.LogoutRequest{
			Cookie: c.cookie,
		}
		payload, err := req.Marshal()
		if err != nil {
			return
		}

		c.Post(payload)
		c.cookie = ""
		c.outDomains = ""
	}
	c.Logger.Info("Logged out\n")
}

// Performs a POST request to the UCS server to create a service profile.
// Returns bool to indicate wether or not the resource could be created,
// along with an error if anything went wrong.
func (c *UCSClient) CreateServiceProfile(sp *ServiceProfile) (bool, error) {
	payload, err := sp.Marshal(c.cookie)
	if err != nil {
		return false, err
	}

	data, err := c.Post(payload)
	if err != nil {
		return false, err
	}

	res, err := ucs.NewServiceProfileResponse(data)
	if err != nil {
		return false, err
	}

	if res.Response == "yes" && res.OutConfigs.ServerConfig.Status == "created" {
		return true, nil
	}

	return false, nil
}

// Determines if the UCSClient is logged into the server by
// checking the presence of cookie.
func (c *UCSClient) IsLoggedIn() bool {
	return len(c.cookie) > 0
}

func getLogFile(filename string) *os.File {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}
	return file
}

func (c *UCSClient) ConfigResolveDN(dn string) (*ServiceProfile, error) {
	//construct the payload
	payload := tplConfigResolveDn(c.cookie, dn)

	//create a new configResolveDn object
	crd := ucs.ConfigResolveDn{}
	//query the UCS
	data, err := c.Post(payload)
	if err != nil {
		return nil, err
	}

	//unmarshal the xml into the object
	err = xml.Unmarshal([]byte(data), &crd)
	if err != nil {
		return nil, err
	}

	if len(crd.OutConfig.ServerConfig) == 0 {
		return nil, fmt.Errorf("No ServerConfig found.")
	}

	//instantiate a new ServiceProfile and fill it with the data
	sp := ServiceProfile{
		Name:      crd.OutConfig.ServerConfig[0].Name,
		Template:  crd.OutConfig.ServerConfig[0].SrcTempl,
		TargetOrg: dn[0:strings.Index(dn, "/")],
		VNICs:     make([]VNIC, 0, 1),
	}

	for _, vnic := range crd.OutConfig.ServerConfig[0].VnicEther {
		sp.VNICs = append(sp.VNICs, VNIC{
			Name: vnic.Name,
			Mac:  vnic.Addr,
		})
	}
	return &sp, nil
}

func (c *UCSClient) endpointURL() string {
	return "https://" + c.ipAddress + "/nuova/"
}

// Find the given xpath in the given document.
func findXPath(xpath string, doc []byte) (string, error) {
	path := xmlpath.MustCompile(xpath)
	rootNode, err := xmlpath.Parse(bytes.NewBuffer(doc))
	if err != nil {
		return "", err
	}

	if value, ok := path.String(rootNode); ok {
		return value, nil
	}

	return "", fmt.Errorf("node blank or not found\n%s", rootNode.String())
}

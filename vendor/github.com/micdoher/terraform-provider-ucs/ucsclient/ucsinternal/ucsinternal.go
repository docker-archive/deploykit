package ucsinternal

import (
	"encoding/xml"
	"strings"
)

const STATUS_DELETED = "deleted"

type (
	ConfigResolveClass struct {
		XMLName    xml.Name   `xml:"configResolveClass"`
		OutConfigs OutConfigs `xml:"outConfigs"`
	}

	ConfigResolveDn struct {
		XMLName   xml.Name  `xml:"configResolveDn"`
		OutConfig OutConfig `xml:"outConfig"`
	}

	ConfigScope struct {
		XMLName    xml.Name `xml:"configScope"`
		OutConfigs OutConfigs
	}

	DestroyRequest struct {
		Name         string
		TargetOrg    string
		Hierarchical bool
	}

	DestroyRequestConfig struct {
		XMLName xml.Name `xml:"inConfigs"`
		Pairs   DestroyRequestPairs
	}

	DestroyRequestPairs struct {
		XMLName xml.Name `xml:"pair"`
		Key     string   `xml:"key,attr"`
		Server  DestroyRequestServerConfig
	}

	DestroyRequestServerConfig struct {
		XMLName xml.Name `xml:"lsServer"`
		Dn      string   `xml:"dn,attr"`
		Status  string   `xml:"status,attr"`
	}

	Dn struct {
		XMLName xml.Name `xml:"dn"`
		Value   string   `xml:"value,attr"`
	}

	InConfig struct {
		XMLName xml.Name `xml:"inConfigs`
		Power   LsPower  `xml:"lsPower,omitempty"`
		Pair    Pair
	}

	InNameSet struct {
		XMLName xml.Name `xml:"inNameSet"`
		Dn      Dn
	}

	LoginRequest struct {
		XMLName  xml.Name `xml:"aaaLogin"`
		Username string   `xml:"inName,attr"`
		Password string   `xml:"inPassword,attr"`
	}

	LogoutRequest struct {
		XMLName xml.Name `xml:"aaaLogout"`
		Cookie  string   `xml:"inCookie,attr"`
	}

	// LogingResponse maps the XML response document returned by UCS when
	// calling the login method.
	LoginResponse struct {
		XMLName    xml.Name `xml:"aaaLogin"`
		Cookie     string   `xml:"cookie,attr"`
		Response   string   `xml:"response,attr"`
		OutCookie  string   `xml:"outCookie,attr"`
		OutDomains string   `xml:"outDomains,attr"`
	}

	LsPower struct {
		Dn    string `xml:"dn,attr"`
		State string `xml:"state,attr"`
	}

	MACPoolAddr struct {
		XMLName  xml.Name `xml:"macpoolAddr"`
		Id       string   `xml:"id,attr"`
		Assigned string   `xml:"assigned,attr"`
		DN       string   `xml:"assignedToDn,attr"`
	}

	OutConfig struct {
		XMLName      xml.Name       `xml:"outConfig"`
		ServerConfig []ServerConfig `xml:"lsServer"`
	}

	OutConfigs struct {
		XMLName      xml.Name      `xml:"outConfigs"`
		MACPoolAddr  []MACPoolAddr `xml:"macpoolAddr"`
		ServerConfig ServerConfig
	}

	Pair struct {
		Key          string       `xml:"key,attr"`
		ServerConfig ServerConfig `xml:",omitempty"`
	}

	ServerConfig struct {
		XMLName   xml.Name    `xml:"lsServer"`
		Dn        string      `xml:"dn,attr"`
		Name      string      `xml:"name,attr"`
		SrcTempl  string      `xml:"srcTemplName,attr"`
		Status    string      `xml:"status,attr"`
		VnicEther []VnicEther `xml:"vnicEther"`
	}

	ServiceProfileRequest struct {
		XMLName         xml.Name `xml:"lsInstantiateNNamedTemplate"`
		Cookie          string   `xml:"cookie,attr"`
		Dn              string   `xml:"dn,attr"`
		TargetOrg       string   `xml:"inTargetOrg,attr"`
		Hierarchical    bool     `xml:"inHierarchical,attr"`
		ErrorOnExisting bool     `xml:"inErrorOnExisting,attr"`
		InNameSet       InNameSet
	}

	ServiceProfileResponse struct {
		XMLName          xml.Name `xml:"lsInstantiateNNamedTemplate"`
		Cookie           string   `xml:"cookie,attr"`
		Dn               string   `xml:"dn,attr"`
		Response         string   `xml:"response,attr"` // YesOrNo
		InvocationResult string   `xml:"invocationResult,attr"`
		ErrorCode        int      `xml:"errorCode,attr"`
		ErrorDescr       string   `xml:"errorDescr,attr"`
		OutConfigs       OutConfigs
	}

	VnicEther struct {
		Addr          string `xml:"addr,attr"`
		IdentPoolName string `xml:"identPoolName,attr"`
		Name          string `xml:"name,attr"`
		NwTemplName   string `xml:"nwTemplName,attr"`
	}

	XMLDestroyRequest struct {
		XMLName        xml.Name `xml:"configConfMos"`
		Cookie         string   `xml:"cookie,attr"`
		InHierarchical bool     `xml:"inHierarchical,attr"`
		InConfig       DestroyRequestConfig
	}
)

// Converts a LoginRequest struct (which contains all the necessary
// login information into a plain-text XML string ready to be
// delivered to the UCS server.
func (req *LoginRequest) Marshal() ([]byte, error) {
	return xml.Marshal(req)
}

// Converts a LogoutRequest struct (which contains all the necessary
// information for logging out into a plain-text XML string ready to be
// delivered to the UCS server.
func (req *LogoutRequest) Marshal() ([]byte, error) {
	return xml.Marshal(req)
}

func (req *DestroyRequest) Marshal(cookie string) ([]byte, error) {
	targetProfile := strings.Join([]string{req.TargetOrg, "/", "ls-", req.Name}, "")
	doc := XMLDestroyRequest{
		Cookie:         cookie,
		InHierarchical: req.Hierarchical,
		InConfig: DestroyRequestConfig{
			Pairs: DestroyRequestPairs{
				Key: targetProfile,
				Server: DestroyRequestServerConfig{
					Dn:     targetProfile,
					Status: STATUS_DELETED,
				},
			},
		},
	}
	return xml.Marshal(doc)
}

// Extracts the login information from a server response (string)
// and maps it into a LoginResponse struct.
func NewLoginResponse(data []byte) (*LoginResponse, error) {
	res := &LoginResponse{}
	err := xml.Unmarshal(data, res)
	return res, err
}

func NewServiceProfileResponse(data []byte) (*ServiceProfileResponse, error) {
	res := &ServiceProfileResponse{}
	err := xml.Unmarshal(data, &res)
	return res, err
}

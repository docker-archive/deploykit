package compute

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// Test that the client can create an instance.
func TestSecurityRulesClient_CreateRule(t *testing.T) {
	server := newAuthenticatingServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Wrong HTTP method %s, expected POST", r.Method)
		}

		expectedPath := "/secrule/"
		if r.URL.Path != expectedPath {
			t.Errorf("Wrong HTTP URL %v, expected %v", r.URL, expectedPath)
		}

		ruleSpec := &SecurityRuleSpec{}
		unmarshalRequestBody(t, r, ruleSpec)

		if ruleSpec.Name != "/Compute-test/test/test-rule1" {
			t.Errorf("Expected name '/Compute-test/test/test-rule1', was %s", ruleSpec.Name)
		}

		if ruleSpec.SourceList != "seciplist:/Compute-test/test/test-list1" {
			t.Errorf("Expected source list 'seciplist:/Compute-test/test/test-list1', was %s",
				ruleSpec.SourceList)
		}

		if ruleSpec.DestinationList != "seclist:/Compute-test/test/test-list2" {
			t.Errorf("Expected destination list 'seclist:/Compute-test/test/test-list2', was %s",
				ruleSpec.DestinationList)
		}

		if ruleSpec.Application != "/oracle/default-application" {
			t.Errorf("Expected application '/oracle/default-application', was %s", ruleSpec.Application)
		}

		w.Write([]byte(exampleCreateSecurityRuleResponse))
		w.WriteHeader(201)
	})

	defer server.Close()
	client := getStubSecurityRulesClient(server)

	info, err := client.CreateSecurityRule(
		"test-rule1",
		"seciplist:test-list1",
		"seclist:test-list2",
		"/oracle/default-application",
		"PERMIT",
		false)

	if err != nil {
		t.Fatalf("Create security rule request failed: %s", err)
	}

	if info.SourceList != "seciplist:es_iplist" {
		t.Errorf("Expected source list 'seciplist:es_iplist', was %s", info.SourceList)
	}

	if info.DestinationList != "seclist:allowed_video_servers" {
		t.Errorf("Expected source list 'seclist:allowed_video_servers', was %s", info.DestinationList)
	}

	if info.Application != "video_streaming_udp" {
		t.Errorf("Expected application 'video_streaming_udp', was %s", info.Application)
	}
}

func getStubSecurityRulesClient(server *httptest.Server) *SecurityRulesClient {
	endpoint, _ := url.Parse(server.URL)
	client := NewComputeClient("test", "test", "test", endpoint)
	authenticatedClient, _ := client.Authenticate()
	return authenticatedClient.SecurityRules()
}

var exampleCreateSecurityRuleResponse = `
{
  "dst_list": "seclist:/Compute-acme/jack.jones@example.com/allowed_video_servers",
  "name": "/Compute-acme/jack.jones@example.com/es_to_videoservers_stream",
  "src_list": "seciplist:/Compute-acme/jack.jones@example.com/es_iplist",
  "uri": "https://api.compute.us0.oraclecloud.com/secrule/Compute-acme/jack.jones@example.com/es_to_videoservers_stream",
  "disabled": false,
  "application": "/Compute-acme/jack.jones@example.com/video_streaming_udp",
  "action": "PERMIT"
}
`

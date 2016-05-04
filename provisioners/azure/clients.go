package azure

import (
	"fmt"
	"github.com/Azure/azure-sdk-for-go/arm/resources/subscriptions"
	"github.com/Azure/go-autorest/autorest"
	"github.com/docker/libmachete/provisioners/api"
	"net/http"
	"time"
)

func subscriptionsClient(baseURI string) subscriptions.Client {
	c := subscriptions.NewClientWithBaseURI(baseURI, "") // used only for unauthenticated requests for generic subs IDs
	c.Client.UserAgent += fmt.Sprintf(";libmachete/%s", api.Version)
	c.RequestInspector = func(p autorest.Preparer) autorest.Preparer {
		return autorest.PreparerFunc(func(r *http.Request) (*http.Request, error) {
			return p.Prepare(r)
		})
	}
	c.ResponseInspector = func(r autorest.Responder) autorest.Responder {
		return autorest.ResponderFunc(func(resp *http.Response) error {
			return r.Respond(resp)
		})
	}
	c.PollingDelay = time.Second * 5
	return c
}

func oauthClient() autorest.Client {
	c := autorest.NewClientWithUserAgent(fmt.Sprintf("libmachete/%s", api.Version))
	c.RequestInspector = func(p autorest.Preparer) autorest.Preparer {
		return autorest.PreparerFunc(func(r *http.Request) (*http.Request, error) {
			return p.Prepare(r)
		})
	}
	c.ResponseInspector = func(r autorest.Responder) autorest.Responder {
		return autorest.ResponderFunc(func(resp *http.Response) error {
			return r.Respond(resp)
		})
	}
	// TODO set user agent
	return c
}

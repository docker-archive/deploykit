package azure

import (
	"fmt"
	"github.com/Azure/azure-sdk-for-go/arm/resources/subscriptions"
	"github.com/Azure/go-autorest/autorest"
	"net/http"
	"time"
)

const version = "0.0"

func subscriptionsClient(baseURI, subscriptionID string) subscriptions.Client {
	// used only for unauthenticated requests for generic subs IDs
	c := subscriptions.NewClientWithBaseURI(baseURI, subscriptionID)
	c.Client.UserAgent += fmt.Sprintf(";libmachete/%s", version)
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
	c := autorest.NewClientWithUserAgent(fmt.Sprintf("libmachete/%s", version))
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

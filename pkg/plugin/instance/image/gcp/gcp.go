package gcp

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/types"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/storage/v1"
)

var log = logutil.New("module", "instance/image/gcp")

const pollingInterval = 500 * time.Millisecond
const timeout = 300

// Client contains state required for communication with GCP
type Client struct {
	client      *http.Client
	compute     *compute.Service
	projectName string
	privKey     *rsa.PrivateKey
}

// NewClient creates a new GCP client
func NewClient(projectName string, serviceAccountKey *types.Any) (*Client, error) {
	log.Debug("connecting", "project", projectName)

	ctx := context.Background()
	var client *Client

	if serviceAccountKey != nil {
		log.Info("Using ServiceAccount key")

		config, err := google.JWTConfigFromJSON(serviceAccountKey.Bytes(),
			storage.DevstorageReadWriteScope,
			compute.ComputeScope,
		)
		if err != nil {
			return nil, err
		}

		client = &Client{
			client:      config.Client(ctx),
			projectName: projectName,
		}
	} else {
		log.Info("Using Application Default credentials")
		gc, err := google.DefaultClient(
			ctx,
			storage.DevstorageReadWriteScope,
			compute.ComputeScope,
		)
		if err != nil {
			return nil, err
		}
		client = &Client{
			client:      gc,
			projectName: projectName,
		}
	}

	var err error
	client.compute, err = compute.New(client.client)
	if err != nil {
		return nil, err
	}

	log.Debug("Generating SSH Keypair")
	client.privKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// CreateImage creates a GCP image using the a source from Google Storage
func (g Client) CreateImage(name, storageURL, family string, replace bool) error {
	if replace {
		if err := g.DeleteImage(name); err != nil {
			return err
		}
	}

	log.Info("Creating image", "name", name)
	imgObj := &compute.Image{
		RawDisk: &compute.ImageRawDisk{
			Source: storageURL,
		},
		Name: name,
	}

	if family != "" {
		imgObj.Family = family
	}

	op, err := g.compute.Images.Insert(g.projectName, imgObj).Do()
	if err != nil {
		return err
	}

	if err := g.pollOperationStatus(op.Name); err != nil {
		return err
	}
	log.Info("created", "image", name)
	return nil
}

// DeleteImage deletes and image
func (g Client) DeleteImage(name string) error {
	var notFound bool
	op, err := g.compute.Images.Delete(g.projectName, name).Do()
	if err != nil {
		if _, ok := err.(*googleapi.Error); !ok {
			return err
		}
		if err.(*googleapi.Error).Code != 404 {
			return err
		}
		notFound = true
	}
	if !notFound {
		log.Info("deleting existing", "image", name)
		if err := g.pollOperationStatus(op.Name); err != nil {
			return err
		}
		log.Info("deleted", "image", name)
	}
	return nil
}

func (g *Client) pollOperationStatus(operationName string) error {
	for i := 0; i < timeout; i++ {
		operation, err := g.compute.GlobalOperations.Get(g.projectName, operationName).Do()
		if err != nil {
			return fmt.Errorf("error fetching operation status: %v", err)
		}
		if operation.Error != nil {
			return fmt.Errorf("error running operation: %v", operation.Error)
		}
		if operation.Status == "DONE" {
			return nil
		}
		time.Sleep(pollingInterval)
	}
	return fmt.Errorf("timeout waiting for operation to finish")

}

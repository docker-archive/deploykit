package s3

import (
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/graymeta/stow"
	"github.com/pkg/errors"
)

// A location contains a client + the configurations used to create the client.
type location struct {
	config stow.Config
	client *s3.S3
}

// CreateContainer creates a new container, in this case an S3 bucket.
// The bare minimum needed is a container name, but there are many other
// options that can be provided.
func (l *location) CreateContainer(containerName string) (stow.Container, error) {
	createBucketParams := &s3.CreateBucketInput{
		Bucket: aws.String(containerName), // required
	}

	_, err := l.client.CreateBucket(createBucketParams)
	if err != nil {
		return nil, errors.Wrap(err, "CreateContainer, creating the bucket")
	}

	region, _ := l.config.Config("region")

	newContainer := &container{
		name:   containerName,
		client: l.client,
		region: region,
	}

	return newContainer, nil
}

// Containers returns a slice of the Container interface, a cursor, and an error.
// This doesn't seem to exist yet in the API without doing a ton of manual work.
// Get the list of buckets, query every single one to retrieve region info, and finally
// return the list of containers that have a matching region against the client. It's not
// possible to manipulate a container within a region that doesn't match the clients'.
// This is because AWS user credentials can be tied to regions. One solution would be
// to start a new client for every single container where the region matches, this would
// also check the credentials on every new instance... Tabled for later.
func (l *location) Containers(prefix, cursor string, count int) ([]stow.Container, string, error) {
	var params *s3.ListBucketsInput

	var containers []stow.Container

	// Response returns exported Owner(*s3.Owner) and Bucket(*s3.[]Bucket)
	bucketList, err := l.client.ListBuckets(params)
	if err != nil {
		return nil, "", errors.Wrap(err, "Containers, listing the buckets")
	}

	// Iterate through the slice of pointers to buckets
	for _, bucket := range bucketList.Buckets {
		// Retrieve region information.
		bucketLocParams := &s3.GetBucketLocationInput{
			Bucket: aws.String(*bucket.Name),
		}

		// Buckets with region 'US Standard' return nothing.
		bucketLocResponse, err := l.client.GetBucketLocation(bucketLocParams)
		if err != nil {
			return nil, "", errors.Wrap(err, "Containers, getting the bucket location")
		}

		clientRegion, _ := l.config.Config("region")
		containerRegion := bucketLocResponse.LocationConstraint

		// If containerRegion (* string) is nil, the region is US Standard, which is "us-east-1"
		// by default.
		if containerRegion == nil {
			usStandardRegion := "us-east-1"
			containerRegion = &usStandardRegion
		}

		// Add buckets with 'US Standard' region. The containerRegion, a pointer, will return nil.
		// Also add buckets that have the same region as the client, otherwise continue on.
		// The second condition ensures that the bucket contains the given prefix.
		if *containerRegion != clientRegion || !strings.HasPrefix(*bucket.Name, prefix) {
			continue
		}

		newContainer := &container{
			name:   *(bucket.Name),
			client: l.client,
			region: clientRegion,
		}

		containers = append(containers, newContainer)
	}

	return containers, "", nil
}

// Close simply satisfies the Location interface. There's nothing that
// needs to be done in order to satisfy the interface.
func (l *location) Close() error {
	return nil // nothing to close
}

// Container retrieves a stow.Container based on its name which must be
// exact.
func (l *location) Container(id string) (stow.Container, error) {
	params := &s3.GetBucketLocationInput{
		Bucket: aws.String(id), // Required
	}

	_, err := l.client.GetBucketLocation(params)
	if err != nil {
		// stow needs ErrNotFound to pass the test but amazon returns an opaque error
		if strings.Contains(err.Error(), "NoSuchBucket") {
			return nil, stow.ErrNotFound
		}
		return nil, errors.Wrap(err, "Container, getting the bucket location")
	}

	region, _ := l.config.Config("region")

	c := &container{
		name:   id,
		client: l.client,
		region: region,
	}

	return c, nil
}

// RemoveContainer removes a container simply by name.
func (l *location) RemoveContainer(id string) error {
	params := &s3.DeleteBucketInput{
		Bucket: aws.String(id),
	}

	_, err := l.client.DeleteBucket(params)
	if err != nil {
		return errors.Wrap(err, "RemoveContainer, deleting the bucket")
	}

	return nil
}

// ItemByURL retrieves a stow.Item by parsing the URL, in this
// case an item is an object.
func (l *location) ItemByURL(url *url.URL) (stow.Item, error) {
	genericURL := []string{"https://s3-", ".amazonaws.com/"}

	// Remove genericURL[0] from URL:
	// url = <genericURL[0]><region><genericURL[1]><bucket name><object path>
	firstCut := strings.Replace(url.Path, genericURL[0], "", 1)

	// find first dot so that we could extract region.
	dotIndex := strings.Index(firstCut, ".")

	// region of the s3 bucket.
	region := firstCut[0:dotIndex]

	// Remove <region><genericURL[1]> from
	// <region><genericURL[1]><bucket name><object path>
	secondCut := strings.Replace(firstCut, region+genericURL[1], "", 1)

	// Get the index of the first slash to get the end of the bucket name.
	firstSlash := strings.Index(secondCut, "/")

	// Grab bucket name
	bucketName := secondCut[:firstSlash]

	// Everything afterwards pertains to object.
	objectPath := secondCut[firstSlash+1:]

	// Get the container by bucket name.
	cont, err := l.Container(bucketName)
	if err != nil {
		return nil, errors.Wrapf(err, "ItemByURL, getting container by the bucketname %v", bucketName)
	}

	// Get the item by object name.
	it, err := cont.Item(objectPath)
	if err != nil {
		return nil, errors.Wrapf(err, "ItemByURL, getting item by object name %v", objectPath)
	}

	return it, err
}

package instance

import (
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/docker/infrakit/pkg/spi/instance"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func randomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func arnOrNameToName(arnOrName string) string {
	if strings.HasPrefix(arnOrName, "arn:aws:") {
		if lastSlashIndex := strings.LastIndex(arnOrName, "/"); lastSlashIndex != -1 {
			return arnOrName[lastSlashIndex+1:]
		}
		return arnOrName[strings.LastIndex(arnOrName, ":")+1:]
	}
	return arnOrName
}

func ec2CreateTags(client ec2iface.EC2API, id instance.ID, tags ...map[string]string) error {
	ec2Tags := []*ec2.Tag{}
	for _, t := range tags {
		for k, v := range t {
			ec2Tags = append(ec2Tags, &ec2.Tag{Key: aws.String(k), Value: aws.String(v)})
		}
	}
	_, err := client.CreateTags(&ec2.CreateTagsInput{Resources: []*string{aws.String(string(id))}, Tags: ec2Tags})
	return err
}

var iamNameProhibitedCharRegexp = regexp.MustCompile(`[^\w+=,.@-]`)

func newIamName(tags ...map[string]string) string {
	name := newUnrestrictedName(tags...)
	return iamNameProhibitedCharRegexp.ReplaceAllString(name, "-")
}

var iamPathProhibitedCharRegexp = regexp.MustCompile(`[^\x21-\x7f]`)

func newIamPath(tags ...map[string]string) string {
	name := newUnrestrictedName(tags...)
	if name == "" {
		return "/"
	}
	name = iamPathProhibitedCharRegexp.ReplaceAllString(name, "-")
	return "/" + name + "/"
}

var loadBalancerNameProhibitedCharRegexp = regexp.MustCompile(`[^a-zA-Z0-9-]`)

func newLoadBalancerName(tags ...map[string]string) string {
	name := newUnrestrictedName(tags...)
	return loadBalancerNameProhibitedCharRegexp.ReplaceAllString(name, "-")
}

var queueNameProhibitedCharRegexp = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func newQueueName(tags ...map[string]string) string {
	name := newUnrestrictedName(tags...)
	return queueNameProhibitedCharRegexp.ReplaceAllString(name, "-")
}

var tableNameProhibitedCharRegexp = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)

func newTableName(tags ...map[string]string) string {
	name := newUnrestrictedName(tags...)
	return tableNameProhibitedCharRegexp.ReplaceAllString(name, "-")
}

func newUnrestrictedName(tags ...map[string]string) string {
	allTags := map[string]string{}
	for _, t := range tags {
		for k, v := range t {
			allTags[k] = v
		}
	}

	keys := []string{}
	for k := range allTags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := []string{}
	for _, k := range keys {
		parts = append(parts, allTags[k])
	}

	return strings.Join(parts, "_")
}

func retry(duration time.Duration, sleep time.Duration, f func() error) error {
	stop := time.Now().Add(duration)
	for {
		if err := f(); err == nil || time.Now().After(stop) {
			return err
		}
		time.Sleep(sleep)
	}
}

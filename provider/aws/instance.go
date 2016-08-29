package aws

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/instance"
	"github.com/docker/libmachete/spi/util/sshutil"
	"github.com/spf13/afero"
	"golang.org/x/crypto/ssh"
	"sort"
	"time"
)

const (
	// ClusterTag is the AWS tag name used to track instances managed by this machete instance.
	ClusterTag = "docker-machete"

	// GroupTag is the AWS tag name used to track instances included in a group.
	GroupTag = "machete-group"

	// VolumeTag is the AWS tag name used to associate unique identifiers (instance.VolumeID) with volumes.
	VolumeTag = "docker-machete-volume"
)

// Provisioner is an instance provisioner for AWS.
type Provisioner struct {
	Client        ec2iface.EC2API
	Cluster       spi.ClusterID
	CommandRunner sshutil.CommandRunner
	KeyStore      sshutil.KeyStore
}

// NewInstanceProvisioner creates a new provisioner using an SSH command runner and local private key storage.
func NewInstanceProvisioner(client ec2iface.EC2API, cluster spi.ClusterID) instance.Provisioner {
	return &Provisioner{
		Client:        client,
		Cluster:       cluster,
		CommandRunner: sshutil.NewCommandRunner(),
		KeyStore:      sshutil.FileSystemKeyStore(afero.NewOsFs(), "./"),
	}
}

func (p Provisioner) tagInstance(request CreateInstanceRequest, instance *ec2.Instance) error {
	tags := []*ec2.Tag{}

	// Gather the tag keys in sorted order, to provide predictable tag order.  This is
	// particularly useful for tests.
	var keys []string
	for k := range request.Tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		key := k
		value := request.Tags[key]
		tags = append(tags, &ec2.Tag{Key: &key, Value: &value})
	}

	// Add cluster and group tags
	tags = append(
		tags,
		&ec2.Tag{Key: aws.String(ClusterTag), Value: aws.String(string(p.Cluster))},
		&ec2.Tag{Key: aws.String(GroupTag), Value: aws.String(string(request.Group))})

	_, err := p.Client.CreateTags(&ec2.CreateTagsInput{Resources: []*string{instance.InstanceId}, Tags: tags})
	return err
}

// CreateInstanceRequest is the concrete provision request type.
type CreateInstanceRequest struct {
	Group             instance.GroupID      `json:"group"`
	Tags              map[string]string     `json:"tags"`
	RunInstancesInput ec2.RunInstancesInput `json:"run_instances_input"`
}

// Provision creates a new instance.
func (p Provisioner) Provision(req string, volume *instance.VolumeID) (*instance.ID, error) {
	request := CreateInstanceRequest{}
	err := json.Unmarshal([]byte(req), &request)
	if err != nil {
		return nil, spi.NewError(spi.ErrBadInput, fmt.Sprintf("Invalid input formatting: %s", err))
	}

	if request.Group == "" {
		return nil, spi.NewError(spi.ErrBadInput, "'group' field must not be blank")
	}

	request.RunInstancesInput.MinCount = aws.Int64(1)
	request.RunInstancesInput.MaxCount = aws.Int64(1)

	var awsVolumeID *string
	if volume != nil {
		volumes, err := p.Client.DescribeVolumes(&ec2.DescribeVolumesInput{
			Filters: []*ec2.Filter{
				clusterFilter(p.Cluster),
				{
					Name:   aws.String(fmt.Sprintf("tag:%s", VolumeTag)),
					Values: []*string{aws.String(string(*volume))},
				},
			},
		})
		if err != nil {
			return nil, spi.NewError(spi.ErrUnknown, "Failed while looking up volume")
		}

		switch len(volumes.Volumes) {
		case 0:
			return nil, spi.NewError(spi.ErrBadInput, fmt.Sprintf("Volume %s does not exist", *volume))
		case 1:
			awsVolumeID = volumes.Volumes[0].VolumeId
		default:
			return nil, spi.NewError(spi.ErrBadInput, "Multiple volume matches found")
		}
	}

	if request.RunInstancesInput.KeyName == nil || *request.RunInstancesInput.KeyName == "" {
		// A custom key was not specified, create one and manage it locally.

		request.RunInstancesInput.KeyName = aws.String(fmt.Sprintf("machete-%s", randomString(5)))

		result, err := p.Client.CreateKeyPair(
			&ec2.CreateKeyPairInput{KeyName: request.RunInstancesInput.KeyName})
		if err != nil {
			return nil, err
		}

		// TODO(wfarner): Need to re-evaluate code paths where keys should be deleted.
		err = p.KeyStore.Write(*request.RunInstancesInput.KeyName, []byte(*result.KeyMaterial))
		if err != nil {
			return nil, err
		}
	}

	reservation, err := p.Client.RunInstances(&request.RunInstancesInput)
	if err != nil {
		// TODO(wfarner): Need to disambiguate between error types.  If there is uncertainty over whether the
		// instance was _actually_ created, we should not delete the key.
		p.KeyStore.Delete(*request.RunInstancesInput.KeyName)
		return nil, err
	}

	if reservation == nil || len(reservation.Instances) != 1 {
		return nil, spi.NewError(spi.ErrUnknown, "Unexpected AWS API response")
	}
	ec2Instance := reservation.Instances[0]

	id := (*instance.ID)(ec2Instance.InstanceId)

	err = p.tagInstance(request, ec2Instance)
	if err != nil {
		return id, err
	}

	if awsVolumeID != nil {
		log.Infof("Waiting for instance %s to enter running state before attaching volume", *id)
		for {
			time.Sleep(10 * time.Second)

			instance, err := p.Client.DescribeInstances(&ec2.DescribeInstancesInput{
				InstanceIds: []*string{ec2Instance.InstanceId},
			})
			if err == nil {
				if *instance.Reservations[0].Instances[0].State.Name == ec2.InstanceStateNameRunning {
					break
				}
			} else if awsErr, ok := err.(awserr.Error); ok {
				if awsErr.Code() == "InvalidInstanceID.NotFound" {
					return id, nil
				}
			}

		}

		_, err := p.Client.AttachVolume(&ec2.AttachVolumeInput{
			InstanceId: ec2Instance.InstanceId,
			VolumeId:   awsVolumeID,
			Device:     aws.String("/dev/sdf"),
		})
		if err != nil {
			return id, err
		}
	}

	return id, nil
}

// Destroy terminates an existing instance.
func (p Provisioner) Destroy(id instance.ID) error {
	describeResult, err := p.describeInstance(id)
	if err != nil {
		return err
	}

	// Only delete the remote key pair if the key is available locally.
	// This enables use of custom or shared keys that are not managed by machete.
	err = p.KeyStore.Delete(*describeResult.KeyName)
	if err == nil {
		_, err = p.Client.DeleteKeyPair(
			&ec2.DeleteKeyPairInput{KeyName: describeResult.KeyName})
		if err != nil {
			return err
		}
	}

	result, err := p.Client.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: []*string{aws.String(string(id))}})

	if err != nil {
		return err
	}

	if len(result.TerminatingInstances) != 1 {
		// There was no match for the instance ID.
		return spi.NewError(spi.ErrBadInput, "No matching instance")
	}

	return nil
}

func clusterFilter(cluster spi.ClusterID) *ec2.Filter {
	// TODO(wfarner): Share these filter definitions with the bootstrap routine.
	return &ec2.Filter{
		Name:   aws.String(fmt.Sprintf("tag:%s", ClusterTag)),
		Values: []*string{aws.String(string(cluster))},
	}
}

func describeGroupRequest(cluster spi.ClusterID, id instance.GroupID, nextToken *string) *ec2.DescribeInstancesInput {
	return &ec2.DescribeInstancesInput{
		NextToken: nextToken,
		Filters: []*ec2.Filter{
			clusterFilter(cluster),
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", GroupTag)),
				Values: []*string{aws.String(string(id))},
			},
			{
				Name: aws.String("instance-state-name"),
				Values: []*string{
					aws.String("pending"),
					aws.String("running"),
				},
			},
		},
	}
}

func (p Provisioner) describeInstances(group instance.GroupID, nextToken *string) ([]instance.Description, error) {
	result, err := p.Client.DescribeInstances(describeGroupRequest(p.Cluster, group, nextToken))
	if err != nil {
		return nil, err
	}

	descriptions := []instance.Description{}
	for _, reservation := range result.Reservations {
		for _, ec2Instance := range reservation.Instances {
			descriptions = append(descriptions, instance.Description{
				ID:               instance.ID(*ec2Instance.InstanceId),
				PrivateIPAddress: *ec2Instance.PrivateIpAddress,
			})
		}
	}

	if result.NextToken != nil {
		// There are more pages of results.
		remainingPages, err := p.describeInstances(group, result.NextToken)
		if err != nil {
			return nil, err
		}

		descriptions = append(descriptions, remainingPages...)
	}

	return descriptions, nil
}

// DescribeInstances implements instance.Provisioner.DescribeInstances.
func (p Provisioner) DescribeInstances(group instance.GroupID) ([]instance.Description, error) {
	return p.describeInstances(group, nil)
}

func (p Provisioner) describeInstance(id instance.ID) (*ec2.Instance, error) {
	result, err := p.Client.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(string(id))},
	})
	if err != nil {
		return nil, err
	}
	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return nil, spi.NewError(spi.ErrBadInput, "Instance not found")
	}

	return result.Reservations[0].Instances[0], nil
}

// ShellExec implements instance.ShellExec.
func (p Provisioner) ShellExec(id instance.ID, shellCode string) (*string, error) {
	instance, err := p.describeInstance(id)
	if err != nil {
		return nil, err
	}

	privateKeyData, err := p.KeyStore.Read(*instance.KeyName)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(privateKeyData)
	if err != nil {
		fmt.Println("Invalid key", string(privateKeyData))
		return nil, err
	}

	config := &ssh.ClientConfig{
		// TODO(wfarner): Figure out how to use the user appropriate for the machine/image.
		User: "ubuntu",
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
	}

	// TODO(wfarner): We probably want the private IP in most cases (and may have no choice, if the host
	// only has a private IP).  Need to figure out what to do here and whether the caller should choose.
	return p.CommandRunner.Exec(fmt.Sprintf("%s:22", *instance.PublicIpAddress), config, shellCode)
}

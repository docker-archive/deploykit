package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/docker/libmachete/controller/quorum"
	machete_aws "github.com/docker/libmachete/provider/aws"
	"github.com/docker/libmachete/spi"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"text/template"
	"time"
)

var (
	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

func addToS3(config client.ConfigProvider, swim fakeSWIMSchema) (*string, error) {
	swimData, err := json.Marshal(swim)
	if err != nil {
		return nil, err
	}

	s3Client := s3.New(config)

	bucket := aws.String(machete_aws.ClusterTag)
	head := &s3.HeadBucketInput{Bucket: bucket}

	_, err = s3Client.HeadBucket(head)
	if err != nil {
		// The bucket does not appear to exist.  Try to create it.
		bucketCreateResult, err := s3Client.CreateBucket(&s3.CreateBucketInput{
			Bucket: bucket,
		})
		if err != nil {
			return nil, err
		}

		log.Infof("Created S3 bucket: %s", bucketCreateResult.Location)

		err = s3Client.WaitUntilBucketExists(head)
		if err != nil {
			return nil, err
		}
	}

	key := aws.String(fmt.Sprintf("%s/config.swim", swim.ClusterName))

	// Note - this will overwrite an existing object.
	putRequest, _ := s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: bucket,
		Key:    key,
		// TODO(wfarner): Explore tightening permissions, as these URLs are reasonably guessable and could
		// potentially contain sensitive information in the future.
		ACL:         aws.String("public-read"),
		Body:        bytes.NewReader(swimData),
		ContentType: aws.String("application/json"),
	})

	err = putRequest.Send()
	if err != nil {
		return nil, err
	}

	return aws.String(putRequest.HTTPRequest.URL.String()), nil
}

func createEBSVolumes(config client.ConfigProvider, swim fakeSWIMSchema) error {
	ec2Client := ec2.New(config)

	for _, managerIP := range swim.ManagerIPs {
		volume, err := ec2Client.CreateVolume(&ec2.CreateVolumeInput{
			AvailabilityZone: aws.String(swim.availabilityZone()),
			Size:             aws.Int64(4),
		})
		if err != nil {
			return err
		}

		log.Infof("Created volume %s", *volume.VolumeId)

		_, err = ec2Client.CreateTags(&ec2.CreateTagsInput{
			Resources: []*string{volume.VolumeId},
			Tags: []*ec2.Tag{
				swim.resourceTag(),
				{
					Key:   aws.String("manager"),
					Value: aws.String(managerIP),
				},
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func applySubnetAndSecurityGroups(run *ec2.RunInstancesInput, subnetID *string, securityGroupIDs ...*string) {
	if run.NetworkInterfaces == nil || len(run.NetworkInterfaces) == 0 {
		run.SubnetId = subnetID
		run.SecurityGroupIds = securityGroupIDs
	} else {
		run.NetworkInterfaces[0].SubnetId = subnetID
		run.NetworkInterfaces[0].Groups = securityGroupIDs
	}
}

func createInternetGateway(ec2Client ec2iface.EC2API, vpcID string, swim fakeSWIMSchema) (*ec2.InternetGateway, error) {
	internetGateway, err := ec2Client.CreateInternetGateway(&ec2.CreateInternetGatewayInput{})
	if err != nil {
		return nil, err
	}

	_, err = ec2Client.AttachInternetGateway(&ec2.AttachInternetGatewayInput{
		VpcId:             aws.String(vpcID),
		InternetGatewayId: internetGateway.InternetGateway.InternetGatewayId,
	})
	if err != nil {
		return nil, err
	}

	return internetGateway.InternetGateway, nil
}

func createRouteTable(
	ec2Client ec2iface.EC2API,
	vpcID string,
	swim fakeSWIMSchema) (*ec2.RouteTable, *ec2.InternetGateway, error) {

	internetGateway, err := createInternetGateway(ec2Client, vpcID, swim)
	if err != nil {
		return nil, nil, err
	}
	log.Infof("Created internet gateway %s", *internetGateway.InternetGatewayId)

	routeTable, err := ec2Client.CreateRouteTable(&ec2.CreateRouteTableInput{VpcId: aws.String(vpcID)})
	if err != nil {
		return nil, nil, err
	}
	log.Infof("Created route table %s", *routeTable.RouteTable.RouteTableId)

	// Route to the internet via the internet gateway.
	_, err = ec2Client.CreateRoute(&ec2.CreateRouteInput{
		RouteTableId:         routeTable.RouteTable.RouteTableId,
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            internetGateway.InternetGatewayId,
	})
	if err != nil {
		return nil, nil, err
	}

	return routeTable.RouteTable, internetGateway, nil
}

func createNetwork(config client.ConfigProvider, swim *fakeSWIMSchema) (string, error) {

	// Apply the private IP address wildcard to the manager.
	if swim.ManagerInstance.RunInstancesInput.NetworkInterfaces == nil ||
		len(swim.ManagerInstance.RunInstancesInput.NetworkInterfaces) == 0 {

		swim.ManagerInstance.RunInstancesInput.PrivateIpAddress = aws.String("{{.IP}}")
	} else {
		swim.ManagerInstance.RunInstancesInput.NetworkInterfaces[0].PrivateIpAddress = aws.String("{{.IP}}")
	}

	ec2Client := ec2.New(config)

	vpc, err := ec2Client.CreateVpc(&ec2.CreateVpcInput{CidrBlock: aws.String("192.168.0.0/16")})
	if err != nil {
		return "", err
	}
	vpcID := *vpc.Vpc.VpcId

	log.Infof("Waiting until VPC %s is available", vpcID)
	err = ec2Client.WaitUntilVpcAvailable(&ec2.DescribeVpcsInput{VpcIds: []*string{vpc.Vpc.VpcId}})
	if err != nil {
		return "", err
	}

	_, err = ec2Client.ModifyVpcAttribute(&ec2.ModifyVpcAttributeInput{
		VpcId:            vpc.Vpc.VpcId,
		EnableDnsSupport: &ec2.AttributeBooleanValue{Value: aws.Bool(true)},
	})
	if err != nil {
		return "", err
	}

	// The API does not allow enabling DnsSupport and DnsHostnames in the same request, so a second modification
	// is made for DnsHostnames.
	_, err = ec2Client.ModifyVpcAttribute(&ec2.ModifyVpcAttributeInput{
		VpcId:              vpc.Vpc.VpcId,
		EnableDnsHostnames: &ec2.AttributeBooleanValue{Value: aws.Bool(true)},
	})
	if err != nil {
		return "", err
	}

	workerSubnet, err := ec2Client.CreateSubnet(&ec2.CreateSubnetInput{
		VpcId:            aws.String(vpcID),
		CidrBlock:        aws.String("192.168.34.0/24"),
		AvailabilityZone: aws.String(swim.availabilityZone()),
	})
	if err != nil {
		return "", err
	}
	log.Infof("Created worker subnet %s", *workerSubnet.Subnet.SubnetId)

	managerSubnet, err := ec2Client.CreateSubnet(&ec2.CreateSubnetInput{
		VpcId:            aws.String(vpcID),
		CidrBlock:        aws.String("192.168.33.0/24"),
		AvailabilityZone: aws.String(swim.availabilityZone()),
	})
	if err != nil {
		return "", err
	}
	log.Infof("Created manager subnet %s", *managerSubnet.Subnet.SubnetId)

	workerGroupRequest := ec2.CreateSecurityGroupInput{
		GroupName:   aws.String("WorkerSecurityGroup"),
		VpcId:       aws.String(vpcID),
		Description: aws.String("Worker node network rules"),
	}
	workerSecurityGroup, err := ec2Client.CreateSecurityGroup(&workerGroupRequest)
	if err != nil {
		return "", err
	}
	log.Infof("Created worker security group %s", *workerSecurityGroup.GroupId)

	managerGroupRequest := ec2.CreateSecurityGroupInput{
		GroupName:   aws.String("ManagerSecurityGroup"),
		VpcId:       aws.String(vpcID),
		Description: aws.String("Manager node network rules"),
	}
	managerSecurityGroup, err := ec2Client.CreateSecurityGroup(&managerGroupRequest)
	if err != nil {
		return "", err
	}
	log.Infof("Created manager security group %s", *managerSecurityGroup.GroupId)

	err = configureManagerSecurityGroup(
		ec2Client,
		*managerSecurityGroup.GroupId,
		*managerSubnet.Subnet,
		*workerSubnet.Subnet)
	if err != nil {
		return "", err
	}

	err = configureWorkerSecurityGroup(ec2Client, *workerSecurityGroup.GroupId, *managerSubnet.Subnet)
	if err != nil {
		return "", err
	}

	routeTable, internetGateway, err := createRouteTable(ec2Client, vpcID, *swim)
	if err != nil {
		return "", err
	}

	_, err = ec2Client.AssociateRouteTable(&ec2.AssociateRouteTableInput{
		SubnetId:     workerSubnet.Subnet.SubnetId,
		RouteTableId: routeTable.RouteTableId,
	})
	if err != nil {
		return "", err
	}

	_, err = ec2Client.AssociateRouteTable(&ec2.AssociateRouteTableInput{
		SubnetId:     managerSubnet.Subnet.SubnetId,
		RouteTableId: routeTable.RouteTableId,
	})
	if err != nil {
		return "", err
	}

	// Tag all resources created.
	_, err = ec2Client.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{
			vpc.Vpc.VpcId,
			workerSubnet.Subnet.SubnetId,
			managerSubnet.Subnet.SubnetId,
			managerSecurityGroup.GroupId,
			workerSecurityGroup.GroupId,
			routeTable.RouteTableId,
			internetGateway.InternetGatewayId,
		},
		Tags: []*ec2.Tag{swim.resourceTag()},
	})
	if err != nil {
		return "", err
	}

	applySubnetAndSecurityGroups(
		&swim.ManagerInstance.RunInstancesInput,
		managerSubnet.Subnet.SubnetId,
		managerSecurityGroup.GroupId)
	applySubnetAndSecurityGroups(
		&swim.WorkerInstance.RunInstancesInput,
		workerSubnet.Subnet.SubnetId,
		workerSecurityGroup.GroupId)

	return vpcID, nil
}

func createAccessRole(config client.ConfigProvider, swim *fakeSWIMSchema) error {
	iamClient := iam.New(config)

	// TODO(wfarner): IAM roles are a global concept in AWS, meaning we will probably need to include region
	// in these entities to avoid collisions.
	role, err := iamClient.CreateRole(&iam.CreateRoleInput{
		RoleName: aws.String(swim.roleName()),
		AssumeRolePolicyDocument: aws.String(`{
			"Version" : "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Principal": {
					"Service": ["ec2.amazonaws.com"]
				},
				"Action": ["sts:AssumeRole"]
			}]
		}`),
	})
	if err != nil {
		return err
	}

	log.Infof("Created IAM role %s (id %s)", *role.Role.RoleName, *role.Role.RoleId)

	policy, err := iamClient.CreatePolicy(&iam.CreatePolicyInput{
		PolicyName: aws.String(swim.managerPolicyName()),

		PolicyDocument: aws.String(`{
			"Version" : "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": "*",
				"Resource": "*"
			}]
		}`),
	})
	if err != nil {
		return err
	}
	log.Infof("Created IAM policy %s (id %s)", *policy.Policy.PolicyName, *policy.Policy.PolicyId)

	_, err = iamClient.AttachRolePolicy(&iam.AttachRolePolicyInput{
		RoleName:  role.Role.RoleName,
		PolicyArn: policy.Policy.Arn,
	})

	instanceProfile, err := iamClient.CreateInstanceProfile(&iam.CreateInstanceProfileInput{
		InstanceProfileName: aws.String(swim.instanceProfileName()),
	})
	if err != nil {
		return err
	}
	log.Infof(
		"Created IAM instance profile %s (id %s)",
		*instanceProfile.InstanceProfile.InstanceProfileName,
		*instanceProfile.InstanceProfile.InstanceProfileId)

	err = iamClient.WaitUntilInstanceProfileExists(&iam.GetInstanceProfileInput{
		InstanceProfileName: instanceProfile.InstanceProfile.InstanceProfileName,
	})
	if err != nil {
		return err
	}

	_, err = iamClient.AddRoleToInstanceProfile(&iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: instanceProfile.InstanceProfile.InstanceProfileName,
		RoleName:            role.Role.RoleName,
	})
	if err != nil {
		return err
	}

	// TODO(wfarner): The above wait does not seem to be sufficient.  Despite apparently waiting for the instance
	// profile to exist, we still encounter an error:
	// "InvalidParameterValue: Value (arn:aws:iam::041673875206:instance-profile/bill-testing-ManagerProfile) for parameter iamInstanceProfile.arn is invalid. Invalid IAM Instance Profile ARN"
	// The same is true of adding a role to an instance profile:
	// InvalidParameterValue: IAM Instance Profile "arn:aws:iam::041673875206:instance-profile/bill-testing-ManagerProfile" has no associated IAM Roles
	// Looks like we may need to poll for the role association as well.
	time.Sleep(10 * time.Second)

	swim.ManagerInstance.RunInstancesInput.IamInstanceProfile = &ec2.IamInstanceProfileSpecification{
		Arn: instanceProfile.InstanceProfile.Arn,
	}

	return err
}

func configureManagerSecurityGroup(
	ec2Client ec2iface.EC2API,
	groupID string,
	managerSubnet ec2.Subnet,
	workerSubnet ec2.Subnet) error {

	// Authorize traffic from worker nodes.
	_, err := ec2Client.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    &groupID,
		IpProtocol: aws.String("-1"),
		FromPort:   aws.Int64(-1),
		ToPort:     aws.Int64(-1),
		CidrIp:     workerSubnet.CidrBlock,
	})
	if err != nil {
		return err
	}

	// Authorize traffic between managers.
	_, err = ec2Client.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    &groupID,
		IpProtocol: aws.String("-1"),
		FromPort:   aws.Int64(-1),
		ToPort:     aws.Int64(-1),
		CidrIp:     managerSubnet.CidrBlock,
	})
	if err != nil {
		return err
	}

	// Authorize SSH to managers.
	_, err = ec2Client.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    &groupID,
		IpProtocol: aws.String("tcp"),
		CidrIp:     aws.String("0.0.0.0/0"),
		FromPort:   aws.Int64(22),
		ToPort:     aws.Int64(22),
	})

	return err
}

func configureWorkerSecurityGroup(ec2Client ec2iface.EC2API, groupID string, managerSubnet ec2.Subnet) error {
	// Authorize traffic from manager nodes.
	_, err := ec2Client.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    aws.String(groupID),
		IpProtocol: aws.String("-1"),
		FromPort:   aws.Int64(-1),
		ToPort:     aws.Int64(-1),
		CidrIp:     managerSubnet.CidrBlock,
	})

	return err
}

func startInitialManager(config client.ConfigProvider, swimConfig fakeSWIMSchema) error {
	builder := machete_aws.Builder{Config: config}
	provisioner, err := builder.BuildInstanceProvisioner(spi.ClusterID(swimConfig.ClusterName))
	if err != nil {
		return err
	}

	managerConfig, err := json.Marshal(swimConfig.ManagerInstance)
	if err != nil {
		return err
	}

	parsed, err := template.New("test").Parse(string(managerConfig))
	if err != nil {
		return err
	}

	return quorum.ProvisionManager(provisioner, parsed, swimConfig.ManagerIPs[0])
}

type fakeSWIMSchema struct {
	Driver          string
	ClusterName     string
	ManagerIPs      []string
	NumManagers     int
	NumWorkers      int
	ManagerInstance machete_aws.CreateInstanceRequest
	WorkerInstance  machete_aws.CreateInstanceRequest
}

func applyInstanceDefaults(r *ec2.RunInstancesInput) {
	if r.InstanceType == nil {
		r.InstanceType = aws.String("t2.micro")
	}

	if r.NetworkInterfaces == nil || len(r.NetworkInterfaces) == 0 {
		r.NetworkInterfaces = []*ec2.InstanceNetworkInterfaceSpecification{
			{
				AssociatePublicIpAddress: aws.Bool(true),
				DeleteOnTermination:      aws.Bool(true),
				DeviceIndex:              aws.Int64(0),
			},
		}
	}
}

func (s *fakeSWIMSchema) applyDefaults() {
	bootLeaderLastOctet := 4
	s.ManagerIPs = []string{}
	for i := 0; i < s.NumManagers; i++ {
		s.ManagerIPs = append(s.ManagerIPs, fmt.Sprintf("192.168.33.%d", bootLeaderLastOctet+i))
	}

	s.WorkerInstance.Group = "workers"
	s.ManagerInstance.Group = "managers"

	applyInstanceDefaults(&s.ManagerInstance.RunInstancesInput)
	applyInstanceDefaults(&s.WorkerInstance.RunInstancesInput)
}

func (s *fakeSWIMSchema) validate() error {
	if s.ClusterName == "" {
		return errors.New("Configuration must specify ClusterName")
	}

	if s.NumManagers != 1 && s.NumManagers != 3 && s.NumManagers != 5 {
		return errors.New("NumManagers must be 1, 3, or 5")
	}

	if s.NumWorkers < 1 {
		return errors.New("NumWorkers must be at least 1")
	}

	if s.ManagerInstance.RunInstancesInput.Placement == nil {
		return errors.New("ManagerInstance.run_instance_input.Placement must be set")
	}
	if s.WorkerInstance.RunInstancesInput.Placement == nil {
		return errors.New("WorkerInstance.run_instance_input.Placement must be set")
	}

	if *s.ManagerInstance.RunInstancesInput.Placement.AvailabilityZone == "" {
		return errors.New("ManagerIntance.run_instanceInput.Placement.AvailabilityZone must be set")
	}

	if *s.ManagerInstance.RunInstancesInput.Placement.AvailabilityZone !=
		*s.ManagerInstance.RunInstancesInput.Placement.AvailabilityZone {

		return errors.New("ManagerInstance and WorkerInstance must be in the same AvailabilityZone")
	}

	return nil
}

func (s *fakeSWIMSchema) availabilityZone() string {
	return *s.ManagerInstance.RunInstancesInput.Placement.AvailabilityZone
}

func (s *fakeSWIMSchema) resourceFilter(vpcID string) []*ec2.Filter {
	return []*ec2.Filter{
		{
			Name:   aws.String("vpc-id"),
			Values: []*string{aws.String(vpcID)},
		},
		s.clusterFilter(),
	}
}

func (s *fakeSWIMSchema) clusterFilter() *ec2.Filter {
	return &ec2.Filter{
		Name:   aws.String(fmt.Sprintf("tag:%s", machete_aws.ClusterTag)),
		Values: []*string{aws.String(s.ClusterName)},
	}
}

func (s *fakeSWIMSchema) roleName() string {
	return fmt.Sprintf("%s-ManagerRole", s.ClusterName)
}

func (s *fakeSWIMSchema) managerPolicyName() string {
	return fmt.Sprintf("%s-ManagerPolicy", s.ClusterName)
}

func (s *fakeSWIMSchema) instanceProfileName() string {
	return fmt.Sprintf("%s-ManagerProfile", s.ClusterName)
}

func (s *fakeSWIMSchema) resourceTag() *ec2.Tag {
	return &ec2.Tag{
		Key:   aws.String(machete_aws.ClusterTag),
		Value: aws.String(s.ClusterName),
	}
}

func (s *fakeSWIMSchema) region() string {
	az := s.availabilityZone()
	return az[:len(az)-1]
}

func getAWSClientConfig(swim fakeSWIMSchema) client.ConfigProvider {
	providers := []credentials.Provider{
		&ec2rolecreds.EC2RoleProvider{Client: ec2metadata.New(session.New())},
		&credentials.EnvProvider{},
		&credentials.SharedCredentialsProvider{},
	}

	return session.New(aws.NewConfig().
		WithRegion(swim.region()).
		WithCredentialsChainVerboseErrors(true).
		WithCredentials(credentials.NewChainCredentials(providers)).
		WithLogger(&logger{}))
}

const machineBootCommand = `#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
start_install() {
  if command -v docker >/dev/null
  then
    echo 'Detected existing Docker installation'
  else
    sleep 5
    curl -sSL https://get.docker.com/ | sh
  fi

  docker run \
    --detach \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    --volume /scratch:/scratch \
    wfarner/swarmboot run $(hostname -i) {{.SWIM_URL}}
}

# See https://github.com/docker/docker/issues/23793#issuecomment-237735835 for
# details on why we background/sleep.
start_install &
`

func injectUserData(swim *fakeSWIMSchema, swimURL string) error {
	userDataTemplate, err := template.New("userdata").Parse(machineBootCommand)
	if err != nil {
		return fmt.Errorf("Internal UserData template is invalid: %s", err)
	}

	buffer := bytes.Buffer{}
	err = userDataTemplate.Execute(&buffer, map[string]string{"SWIM_URL": swimURL})
	if err != nil {
		return fmt.Errorf("Failed to populate internal UserData template: %s", err)
	}

	userData := base64.StdEncoding.EncodeToString(buffer.Bytes())
	swim.ManagerInstance.RunInstancesInput.UserData = &userData
	swim.WorkerInstance.RunInstancesInput.UserData = &userData

	return nil
}

func bootstrap(swimFile string) error {
	swimData, err := ioutil.ReadFile(swimFile)
	if err != nil {
		return fmt.Errorf("Failed to read config file: %s", err)
	}

	swim := fakeSWIMSchema{}
	err = json.Unmarshal(swimData, &swim)
	if err != nil {
		return fmt.Errorf("Invalid JSON in SWIM file: %s", err)
	}

	err = swim.validate()
	if err != nil {
		return fmt.Errorf("Invalid SWIM file: %s", err)
	}

	swim.applyDefaults()

	sess := getAWSClientConfig(swim)

	// TODO(wfarner): Integrate setup and attachment of EBS volumes.
	// TODO(wfarner): Figure out a way to format them during bootstrapping as well, since it would be unsafe
	// for the manager nodes to determine whether they should be formatted.
	/*
		err = createEBSVolumes(sess, swimConfig)
		if err != nil {
			return err
		}
	*/

	err = createAccessRole(sess, &swim)
	if err != nil {
		return err
	}

	vpcID, err := createNetwork(sess, &swim)
	if err != nil {
		return err
	}

	// TODO(wfarner): It would be nice to avoid the second upload to S3.  We don't need this if we can predict the
	// S3 URL.
	swimURL, err := addToS3(sess, swim)
	if err != nil {
		return err
	}

	log.Infof("SWIM file URL: %s", *swimURL)

	err = injectUserData(&swim, *swimURL)
	if err != nil {
		return err
	}

	swimURL, err = addToS3(sess, swim)
	if err != nil {
		return err
	}

	// Create one manager instance.  The manager boot container will handle setting up other containers.
	err = startInitialManager(sess, swim)
	if err != nil {
		return err
	}

	ec2Client := ec2.New(sess)
	filter := []*ec2.Filter{{
		Name:   aws.String("instance-state-name"),
		Values: []*string{aws.String("pending"), aws.String("running")},
	}}
	filter = append(filter, swim.resourceFilter(vpcID)...)
	instancesResp, err := ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{Filters: filter})
	if err != nil {
		return fmt.Errorf("Failed to fetch instances: %s", err)
	}
	publicIP := ""
	for _, reservation := range instancesResp.Reservations {
		for _, instance := range reservation.Instances {
			if publicIP == "" {
				if instance.PublicIpAddress == nil {
					log.Warnf(
						"Expected instances to have public IPs but %s does not",
						*instance.InstanceId)
				} else {
					publicIP = *instance.PublicIpAddress
				}
			} else {
				log.Warnf(
					"Expected only one instance in the cluster, also found %s",
					*instance.InstanceId)
			}
		}
	}
	log.Infof("Boot leader has public IP %s", publicIP)

	return nil
}

func destroyInstances(config client.ConfigProvider, swim fakeSWIMSchema, vpcID string) {
	ec2Client := ec2.New(config)

	instancesResp, err := ec2Client.DescribeInstances(
		&ec2.DescribeInstancesInput{Filters: swim.resourceFilter(vpcID)})
	if err != nil {
		log.Errorf("Failed to fetch instances: %s", err)
		return
	}

	instanceIDs := []*string{}
	for _, reservation := range instancesResp.Reservations {
		for _, instance := range reservation.Instances {
			instanceIDs = append(instanceIDs, instance.InstanceId)
		}
	}

	if len(instanceIDs) > 0 {
		nonPointerIds := []string{}
		for _, id := range instanceIDs {
			nonPointerIds = append(nonPointerIds, *id)
		}

		log.Infof("Terminating instances %s", nonPointerIds)
		_, err = ec2Client.TerminateInstances(&ec2.TerminateInstancesInput{InstanceIds: instanceIDs})
		if err != nil {
			log.Errorf("Failed to terminate instances: %s", err)
			return
		}

		// TODO(wfarner): Need a more robust routine here since instances can self-replicate.  For example,
		// the describe/terminate sequence here could race with a manager node trying to reach a target
		// instance count.
		err = ec2Client.WaitUntilInstanceTerminated(&ec2.DescribeInstancesInput{InstanceIds: instanceIDs})
		if err != nil {
			log.Warnf("Error while waiting for instances to terminate: %s", err)
		}
	}
}

func destroyAccessRoles(config client.ConfigProvider, swim fakeSWIMSchema) {
	iamClient := iam.New(config)

	_, err := iamClient.RemoveRoleFromInstanceProfile(&iam.RemoveRoleFromInstanceProfileInput{
		InstanceProfileName: aws.String(swim.instanceProfileName()),
		RoleName:            aws.String(swim.roleName()),
	})
	if err != nil {
		log.Warnf("Error while removing role from instance profile: %s", err)
	}

	log.Infof("Deleting instance profile %s", swim.instanceProfileName())
	_, err = iamClient.DeleteInstanceProfile(&iam.DeleteInstanceProfileInput{
		InstanceProfileName: aws.String(swim.instanceProfileName()),
	})
	if err != nil {
		log.Warnf("Error while deleting instance profile: %s", err)
	}

	// There must be a better way...but i couldn't find another way to look up the policy ARN.
	policies, err := iamClient.ListPolicies(&iam.ListPoliciesInput{
		Scope: aws.String("Local"),
	})
	for _, policy := range policies.Policies {
		if *policy.PolicyName == swim.managerPolicyName() {
			_, err = iamClient.DetachRolePolicy(&iam.DetachRolePolicyInput{
				RoleName:  aws.String(swim.roleName()),
				PolicyArn: policy.Arn,
			})
			if err != nil {
				log.Warnf("Error while detaching IAM role policy: %s", err)
			}

			log.Infof("Deleting IAM policy %s", *policy.Arn)
			_, err = iamClient.DeletePolicy(&iam.DeletePolicyInput{
				PolicyArn: policy.Arn,
			})
			if err != nil {
				log.Warnf("Error while deleting IAM policy: %s", err)
			}
		}
	}

	if err != nil {
		log.Warnf("Error while deleting IAM role policy: %s", err)
	}

	log.Infof("Deleting IAM role %s", swim.roleName())
	_, err = iamClient.DeleteRole(&iam.DeleteRoleInput{RoleName: aws.String(swim.roleName())})
	if err != nil {
		log.Warnf("Error while deleting IAM role: %s", err)
	}
}

func destroyNetwork(config client.ConfigProvider, swim fakeSWIMSchema, vpcID string) {
	ec2Client := ec2.New(config)

	securityGroups, err := ec2Client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: swim.resourceFilter(vpcID),
	})
	if err == nil {
		for _, securityGroup := range securityGroups.SecurityGroups {
			log.Infof("Deleting security group %s", *securityGroup.GroupId)
			_, err = ec2Client.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
				GroupId: securityGroup.GroupId,
			})
			if err != nil {
				log.Warnf("Error while deleting security group: %s", err)
			}
		}
	} else {
		log.Warnf("Error while describing security groups: %s", err)
	}

	subnets, err := ec2Client.DescribeSubnets(&ec2.DescribeSubnetsInput{Filters: []*ec2.Filter{
		{
			Name:   aws.String("vpc-id"),
			Values: []*string{aws.String(vpcID)},
		},
		swim.clusterFilter(),
	}})
	if err == nil {
		for _, subnet := range subnets.Subnets {
			log.Infof("Deleting subnet %s", *subnet.SubnetId)
			_, err = ec2Client.DeleteSubnet(&ec2.DeleteSubnetInput{SubnetId: subnet.SubnetId})
			if err != nil {
				log.Warnf("Error while deleting subnet: %s", err)
			}
		}
	} else {
		log.Warnf("Error while looking up subnets: %s", err)
	}

	internetGateways, err := ec2Client.DescribeInternetGateways(&ec2.DescribeInternetGatewaysInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("attachment.vpc-id"),
				Values: []*string{aws.String(vpcID)},
			},
			swim.clusterFilter(),
		},
	})
	if err == nil {
		for _, internetGateway := range internetGateways.InternetGateways {
			log.Infof(
				"Detaching internet gateway %s from VPC %s",
				*internetGateway.InternetGatewayId,
				vpcID)
			_, err := ec2Client.DetachInternetGateway(&ec2.DetachInternetGatewayInput{
				InternetGatewayId: internetGateway.InternetGatewayId,
				VpcId:             aws.String(vpcID),
			})
			if err != nil {
				log.Warnf("Error detaching internet gateways: %s", err)
			}

			log.Infof("Deleting internet gateway %s", *internetGateway.InternetGatewayId)
			_, err = ec2Client.DeleteInternetGateway(&ec2.DeleteInternetGatewayInput{
				InternetGatewayId: internetGateway.InternetGatewayId,
			})
			if err != nil {
				log.Warnf("Error deleting internet gateways: %s", err)
			}
		}
	} else {
		log.Warnf("Error looking up internet gateways: %s", err)
	}

	routeTables, err := ec2Client.DescribeRouteTables(
		&ec2.DescribeRouteTablesInput{Filters: swim.resourceFilter(vpcID)})
	if err == nil {
		for _, routeTable := range routeTables.RouteTables {
			log.Infof("Deleting route table %s", *routeTable.RouteTableId)
			_, err = ec2Client.DeleteRouteTable(&ec2.DeleteRouteTableInput{
				RouteTableId: routeTable.RouteTableId,
			})
			if err != nil {
				log.Warnf("Error while deleting route table: %s", err)
			}
		}
	} else {
		log.Warnf("Error while describing route tables: %s", err)
	}

	log.Infof("Deleting VPC %s", vpcID)
	_, err = ec2Client.DeleteVpc(&ec2.DeleteVpcInput{VpcId: aws.String(vpcID)})
	if err != nil {
		log.Warnf("Error while deleting VPC: %s", err)
	}
}

func destroy(swimFile string) error {
	swimData, err := ioutil.ReadFile(swimFile)
	if err != nil {
		return fmt.Errorf("Failed to read config file: %s", err)
	}

	swim := fakeSWIMSchema{}
	err = json.Unmarshal(swimData, &swim)
	if err != nil {
		return fmt.Errorf("Invalid JSON in SWIM file: %s", err)
	}

	sess := getAWSClientConfig(swim)

	ec2Client := ec2.New(sess)

	vpcs, err := ec2Client.DescribeVpcs(&ec2.DescribeVpcsInput{Filters: []*ec2.Filter{swim.clusterFilter()}})
	if err != nil {
		return fmt.Errorf("Failed to look up VPC: %s", err)
	}

	// TODO(wfarner): We omit the VPC ID from resource tags and allow more failure-resistant cleanup as long as we
	// disallow clusters of the same name to exist within a region.
	var vpcID string
	switch len(vpcs.Vpcs) {
	case 0:
		log.Warnf("No VPCs found for cluster %s, unable to remove networks or instances", swim.ClusterName)
	case 1:
		vpcID = *vpcs.Vpcs[0].VpcId
	default:
		log.Warnf("Found multiple VPCs for cluster %s, unable to remove networks or instances", swim.ClusterName)
	}

	if vpcID != "" {
		destroyInstances(sess, swim, vpcID)
	}

	destroyAccessRoles(sess, swim)

	if vpcID != "" {
		destroyNetwork(sess, swim, vpcID)
	}

	return nil
}

func main() {
	rootCmd := &cobra.Command{
		Use: "bootstrap",
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "create <swim config>",
		Short: "perform the bootstrap sequence",
		Long:  "bootstrap a swarm cluster using a SWIM configuration",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				cmd.Usage()
				return
			}

			swimFile := args[0]
			err := bootstrap(swimFile)
			if err != nil {
				log.Fatal(err.Error())
				os.Exit(1)
			}
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "destroy <swim config>",
		Short: "destroy a swarm cluster",
		Long:  "destroy all resources associated with a SWIM configuration",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				cmd.Usage()
				return
			}

			swimFile := args[0]
			err := destroy(swimFile)
			if err != nil {
				log.Fatal(err.Error())
				os.Exit(1)
			}
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print build version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s (revision %s)\n", Version, Revision)
		},
	})

	err := rootCmd.Execute()
	if err != nil {
		log.Print(err)
		os.Exit(-1)
	}
}

type logger struct {
}

func (l logger) Log(args ...interface{}) {
	log.Println(args)
}

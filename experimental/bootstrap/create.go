package bootstrap

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/iam"
	infrakit_instance "github.com/docker/infrakit.aws/plugin/instance"
	"github.com/docker/infrakit/spi/group"
	"github.com/docker/infrakit/spi/instance"
	"strings"
	"text/template"
	"time"
)

func createEBSVolumes(config client.ConfigProvider, spec clusterSpec) error {
	log.Info("Creating EBS volumes")
	ec2Client := ec2.New(config)

	volumeIDs := []*string{}
	for _, managerIP := range spec.ManagerIPs {
		volume, err := ec2Client.CreateVolume(&ec2.CreateVolumeInput{
			AvailabilityZone: aws.String(spec.availabilityZone()),
			Size:             aws.Int64(4),
		})
		volumeIDs = append(volumeIDs, volume.VolumeId)
		if err != nil {
			return err
		}

		log.Infof("  %s", *volume.VolumeId)

		_, err = ec2Client.CreateTags(&ec2.CreateTagsInput{
			Resources: []*string{volume.VolumeId},
			Tags: []*ec2.Tag{
				spec.cluster().resourceTag(),
				{
					Key:   aws.String(infrakit_instance.VolumeTag),
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

func createInternetGateway(ec2Client ec2iface.EC2API, vpcID string) (*ec2.InternetGateway, error) {
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

func createRouteTable(ec2Client ec2iface.EC2API, vpcID string) (*ec2.RouteTable, *ec2.InternetGateway, error) {

	internetGateway, err := createInternetGateway(ec2Client, vpcID)
	if err != nil {
		return nil, nil, err
	}
	log.Infof("  internet gateway %s", *internetGateway.InternetGatewayId)

	routeTable, err := ec2Client.CreateRouteTable(&ec2.CreateRouteTableInput{VpcId: aws.String(vpcID)})
	if err != nil {
		return nil, nil, err
	}
	log.Infof("  route table %s", *routeTable.RouteTable.RouteTableId)

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

func createNetwork(config client.ConfigProvider, spec *clusterSpec) (string, error) {
	log.Info("Creating network resources")

	// Apply the private IP address wildcard to the manager.
	spec.mutateManagers(func(managers *instanceGroupSpec) {
		if managers.Config.RunInstancesInput.NetworkInterfaces == nil ||
			len(managers.Config.RunInstancesInput.NetworkInterfaces) == 0 {

			managers.Config.RunInstancesInput.PrivateIpAddress = aws.String("{{.IP}}")
		} else {
			managers.Config.RunInstancesInput.NetworkInterfaces[0].PrivateIpAddress = aws.String("{{.IP}}")
		}
	})

	ec2Client := ec2.New(config)

	vpc, err := ec2Client.CreateVpc(&ec2.CreateVpcInput{CidrBlock: aws.String("192.168.0.0/16")})
	if err != nil {
		return "", err
	}
	vpcID := *vpc.Vpc.VpcId

	log.Infof("  VPC %s, waiting for it to become available", vpcID)
	vpcDescribe := ec2.DescribeVpcsInput{VpcIds: []*string{vpc.Vpc.VpcId}}
	err = ec2Client.WaitUntilVpcExists(&vpcDescribe)
	if err != nil {
		return "", fmt.Errorf("Failed while waiting for VPC to exist - %s", err)
	}

	err = ec2Client.WaitUntilVpcAvailable(&vpcDescribe)
	if err != nil {
		return "", fmt.Errorf("Failed while waiting for VPC to become available - %s", err)
	}

	_, err = ec2Client.ModifyVpcAttribute(&ec2.ModifyVpcAttributeInput{
		VpcId:            vpc.Vpc.VpcId,
		EnableDnsSupport: &ec2.AttributeBooleanValue{Value: aws.Bool(true)},
	})
	if err != nil {
		return "", fmt.Errorf("Failed to modify VPC attribute - %s", err)
	}

	// The API does not allow enabling DnsSupport and DnsHostnames in the same request, so a second modification
	// is made for DnsHostnames.
	_, err = ec2Client.ModifyVpcAttribute(&ec2.ModifyVpcAttributeInput{
		VpcId:              vpc.Vpc.VpcId,
		EnableDnsHostnames: &ec2.AttributeBooleanValue{Value: aws.Bool(true)},
	})
	if err != nil {
		return "", fmt.Errorf("Failed to modify VPC attribute - %s", err)
	}

	workerSubnet, err := ec2Client.CreateSubnet(&ec2.CreateSubnetInput{
		VpcId:            aws.String(vpcID),
		CidrBlock:        aws.String("192.168.34.0/24"),
		AvailabilityZone: aws.String(spec.availabilityZone()),
	})
	if err != nil {
		return "", err
	}
	log.Infof("  worker subnet %s", *workerSubnet.Subnet.SubnetId)

	managerSubnet, err := ec2Client.CreateSubnet(&ec2.CreateSubnetInput{
		VpcId:            aws.String(vpcID),
		CidrBlock:        aws.String("192.168.33.0/24"),
		AvailabilityZone: aws.String(spec.availabilityZone()),
	})
	if err != nil {
		return "", err
	}
	log.Infof("  manager subnet %s", *managerSubnet.Subnet.SubnetId)

	workerGroupRequest := ec2.CreateSecurityGroupInput{
		GroupName:   aws.String("WorkerSecurityGroup"),
		VpcId:       aws.String(vpcID),
		Description: aws.String("Worker node network rules"),
	}
	workerSecurityGroup, err := ec2Client.CreateSecurityGroup(&workerGroupRequest)
	if err != nil {
		return "", err
	}
	log.Infof("  worker security group %s", *workerSecurityGroup.GroupId)

	managerGroupRequest := ec2.CreateSecurityGroupInput{
		GroupName:   aws.String("ManagerSecurityGroup"),
		VpcId:       aws.String(vpcID),
		Description: aws.String("Manager node network rules"),
	}
	managerSecurityGroup, err := ec2Client.CreateSecurityGroup(&managerGroupRequest)
	if err != nil {
		return "", err
	}
	log.Infof("  manager security group %s", *managerSecurityGroup.GroupId)

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

	routeTable, internetGateway, err := createRouteTable(ec2Client, vpcID)
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
		Tags: []*ec2.Tag{spec.cluster().resourceTag()},
	})
	if err != nil {
		return "", err
	}

	spec.mutateGroups(func(group *instanceGroupSpec) {
		if group.isManager() {
			applySubnetAndSecurityGroups(
				&group.Config.RunInstancesInput,
				managerSubnet.Subnet.SubnetId,
				managerSecurityGroup.GroupId)
		} else {
			applySubnetAndSecurityGroups(
				&group.Config.RunInstancesInput,
				workerSubnet.Subnet.SubnetId,
				workerSecurityGroup.GroupId)
		}
	})

	return vpcID, nil
}

func createAccessRole(config client.ConfigProvider, spec *clusterSpec) error {
	log.Info("Creating IAM resources")

	iamClient := iam.New(config)

	// TODO(wfarner): IAM roles are a global concept in AWS, meaning we will probably need to include region
	// in these entities to avoid collisions.
	role, err := iamClient.CreateRole(&iam.CreateRoleInput{
		RoleName: aws.String(spec.cluster().roleName()),
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

	log.Infof("  role %s (id %s)", *role.Role.RoleName, *role.Role.RoleId)

	policy, err := iamClient.CreatePolicy(&iam.CreatePolicyInput{
		PolicyName: aws.String(spec.cluster().managerPolicyName()),

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
	log.Infof("  policy %s (id %s)", *policy.Policy.PolicyName, *policy.Policy.PolicyId)

	_, err = iamClient.AttachRolePolicy(&iam.AttachRolePolicyInput{
		RoleName:  role.Role.RoleName,
		PolicyArn: policy.Policy.Arn,
	})

	instanceProfile, err := iamClient.CreateInstanceProfile(&iam.CreateInstanceProfileInput{
		InstanceProfileName: aws.String(spec.cluster().instanceProfileName()),
	})
	if err != nil {
		return err
	}
	log.Infof(
		"  instance profile %s (id %s), waiting for it to exist",
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

	spec.mutateManagers(func(managers *instanceGroupSpec) {
		managers.Config.RunInstancesInput.IamInstanceProfile = &ec2.IamInstanceProfileSpecification{
			Arn: instanceProfile.InstanceProfile.Arn,
		}
	})

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

// ProvisionManager creates a single manager instance, replacing the IP address wildcard with the provided IP.
func ProvisionManager(
	provisioner instance.Plugin,
	tags map[string]string,
	provisionRequest json.RawMessage,
	ip string) error {

	logicalID := instance.LogicalID(ip)

	id, err := provisioner.Provision(instance.Spec{
		Properties:  &provisionRequest,
		Tags:        tags,
		LogicalID:   &logicalID,
		Attachments: []instance.Attachment{instance.Attachment(ip)},
	})
	if err != nil {
		return fmt.Errorf("Failed to provision: %s", err)
	}

	log.Infof("Provisioned instance %s with IP %s", *id, ip)
	return nil
}

// InstanceTags gets the tags used to associate an instance with a group.
func InstanceTags(resourceTag ec2.Tag, gid group.ID) map[string]string {
	return map[string]string{
		*resourceTag.Key: *resourceTag.Value,
		"infrakit.group": string(gid),
	}
}

const prepareGroupWatches = `
plugins=/infrakit/plugins
configs=/infrakit/configs
discovery="-e INFRAKIT_PLUGINS_DIR=$plugins -v $plugins:$plugins"
run_plugin="docker run -d --restart always $discovery"
image=wfarner/infrakit-demo-plugins

mkdir -p $configs
mkdir -p $plugins

{{ range $name, $config := . }}
cat << 'EOF' > "$configs/{{ $name }}.json"
{{ $config }}
EOF
{{ end }}

docker pull $image
$run_plugin --name flavor-combo $image infrakit-flavor-combo
$run_plugin --name flavor-swarm -v /var/run/docker.sock:/var/run/docker.sock $image infrakit-flavor-swarm
$run_plugin --name flavor-vanilla $image infrakit-flavor-vanilla
$run_plugin --name group-default $image infrakit-group-default
$run_plugin --name instance-aws $image infrakit-instance-aws

echo "alias infrakit='docker run --rm $discovery $image infrakit'" >> /home/ubuntu/.bashrc

{{ range $name, $config := . }}
docker run --rm $discovery -v $configs:$configs $image infrakit group watch $configs/{{ $name }}.json
{{ end }}
`

func startInitialManager(config client.ConfigProvider, spec clusterSpec) error {
	log.Info("Starting cluster boot leader instance")
	builder := infrakit_instance.Builder{Config: config}
	provisioner, err := builder.BuildInstancePlugin()
	if err != nil {
		return err
	}

	managerGroup := spec.managers()

	// Produce InfraKit groups.
	infrakitGroups, err := generateInfraKitGroups(spec)
	if err != nil {
		return err
	}

	buffer := bytes.Buffer{}
	err = template.Must(template.New("").Parse(prepareGroupWatches)).Execute(&buffer, infrakitGroups)
	if err != nil {
		return err
	}

	// TODO(wfarner): Include shell code that creates infrakit group JSON files, and watches the groups.
	managerGroup.Config.RunInstancesInput.UserData = aws.String(strings.Join([]string{
		"#!/bin/bash",
		initializeManager,
		"curl -sSL https://get.docker.com/ | sh",
		"docker swarm init",
		string(buffer.Bytes()),
	}, "\n"))

	rawConfig, err := json.Marshal(managerGroup.Config)
	if err != nil {
		return err
	}

	return ProvisionManager(
		provisioner,
		InstanceTags(*spec.cluster().resourceTag(), managerGroup.Name),
		json.RawMessage(rawConfig),
		spec.ManagerIPs[0])
}

const (
	// TODO(wfarner): Ideally we would have a better indicator of a file system that has not yet been formatted
	// than inspecting the block device.  For examlpe, a trashed file system may be grounds for operator
	// intervention rather than the system deciding to clear its state.
	initializeManager = `
set -o errexit
set -o nounset
set -o xtrace

# See http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/device_naming.html why device naming is tricky, and likely
# coupled to the AMI (host OS) used.
EBS_DEVICE=/dev/xvdf

# Determine whether the EBS volume needs to be formatted.
if [ "$(file -sL $EBS_DEVICE)" = "$EBS_DEVICE: data" ]
then
  echo 'Formatting EBS volume device'
  mkfs -t ext4 $EBS_DEVICE
fi

mkdir -p /var/lib/docker
echo "$EBS_DEVICE /var/lib/docker ext4 defaults,nofail 0 2" >> /etc/fstab
mount -a
`
)

const (
	managerGroup = `{
  "ID": {{.ID}},
  "Properties": {
    "Allocation": {
      "LogicalIDs": {{.ManagerIPs}}
    },
    "Instance": {
      "Plugin": "instance-aws",
      "Properties": {{.CreateInstanceRequest}}
    },
    "Flavor": {
      "Plugin": "flavor-combo",
      "Properties": {
        "Flavors": [
          {
            "Plugin": "flavor-vanilla",
            "Properties": {
              "Init": [
                "#!/bin/bash",
                {{.BootScript}},
                "curl -sSL https://get.docker.com/ | sh"
              ]
            }
          },
          {
            "Plugin": "flavor-swarm",
            "Properties": {
              "Type": "manager",
              "DockerRestartCommand": "systemctl restart docker"
            }
          }
        ]
      }
    }
  }
}`

	workerGroup = `{
  "ID": {{.ID}},
  "Properties": {
    "Allocation": {
      "Size": {{.WorkerCount}}
    },
    "Instance": {
      "Plugin": "instance-aws",
      "Properties": {{.CreateInstanceRequest}}
    },
    "Flavor": {
      "Plugin": "flavor-combo",
      "Properties": {
        "Flavors": [
          {
            "Plugin": "flavor-vanilla",
            "Properties": {
              "Init": [
                "#!/bin/bash",
                "curl -sSL https://get.docker.com/ | sh"
              ]
            }
          },
          {
            "Plugin": "flavor-swarm",
            "Properties": {
              "Type": "worker",
              "DockerRestartCommand": "systemctl restart docker"
            }
          }
        ]
      }
    }
  }
}`
)

func bootstrap(spec clusterSpec) error {
	sess := spec.cluster().getAWSClient()

	keyNames := []*string{}
	for _, g := range spec.Groups {
		keyNames = append(keyNames, g.Config.RunInstancesInput.KeyName)
	}

	ec2Client := ec2.New(sess)
	_, err := ec2Client.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{
		KeyNames: keyNames,
	})
	if err != nil {
		return err
	}

	err = createAccessRole(sess, &spec)
	if err != nil {
		return err
	}

	vpcID, err := createNetwork(sess, &spec)
	if err != nil {
		return err
	}

	err = createEBSVolumes(sess, spec)
	if err != nil {
		return err
	}

	// Create one manager instance.  The manager boot container will handle setting up other containers.
	err = startInitialManager(sess, spec)
	if err != nil {
		return err
	}

	getInstances := func(req *ec2.DescribeInstancesInput) ([]*ec2.Instance, error) {
		instances := []*ec2.Instance{}

		instancesResp, err := ec2Client.DescribeInstances(req)
		if err != nil {
			return nil, err
		}
		for _, reservation := range instancesResp.Reservations {
			instances = append(instances, reservation.Instances...)
		}
		return instances, nil
	}

	instances, err := getInstances(&ec2.DescribeInstancesInput{Filters: spec.cluster().resourceFilter(vpcID)})
	if err != nil {
		return fmt.Errorf("Failed to fetch boot leader: %s", err)
	}
	if len(instances) != 1 {
		log.Warnf("Expected exactly one instance to be starting up, but found %d", len(instances))
		return nil
	}

	// Public IP addresses are assigned some time between when an instance is started and when it enters running.
	// To avoid racing here, we wait until running state to ensure a public IP is assigned.
	log.Infof("Waiting for boot leader to run")
	getBootLeader := ec2.DescribeInstancesInput{InstanceIds: []*string{instances[0].InstanceId}}
	err = ec2Client.WaitUntilInstanceRunning(&getBootLeader)
	if err != nil {
		return fmt.Errorf("Failed while waiting for boot leader to start up: %s", err)
	}

	leaders, err := getInstances(&getBootLeader)
	if err != nil {
		return fmt.Errorf("Failed to fetch boot leader: %s", err)
	}
	if len(leaders) != 1 {
		log.Warnf("Expected exactly one boot leader, but found %s", len(instances))
		return nil
	}

	leader := leaders[0]
	if leader.PublicIpAddress == nil {
		log.Warnf(
			"Expected instances to have public IPs but %s does not",
			*leader.InstanceId)
	} else {
		log.Infof("")
		log.Infof("Your Docker cluster is now booting!")
		log.Infof("")
		log.Infof("It may take a few more minutes for the cluster to be ready, at which point you can SSH")
		log.Infof("to %s using the default login user for the AMI, and the private", *leader.PublicIpAddress)
		log.Infof("SSH key associated with the public key '%s' in AWS.", *leader.KeyName)
		log.Infof("You can see other nodes tha thave joined the cluster by running 'docker node ls'")
	}

	return nil
}

func generateInfraKitGroups(spec clusterSpec) (map[group.ID]string, error) {
	groups := map[group.ID]string{}

	for _, grp := range spec.Groups {
		buffer := bytes.Buffer{}
		templateText := ""
		templateParams := map[string]interface{}{
			"CreateInstanceRequest": grp.Config,
			"ID": grp.Name,
		}

		if grp.isManager() {
			templateText = managerGroup
			templateParams["ManagerIPs"] = spec.ManagerIPs
			templateParams["BootScript"] = initializeManager
		} else {
			templateText = workerGroup
			templateParams["WorkerCount"] = grp.Size
		}

		// Convert all template parameters to JSON.
		for k, v := range templateParams {
			vJSON, err := json.MarshalIndent(v, "      ", "  ")
			if err != nil {
				return nil, err
			}

			templateParams[k] = string(vJSON)
		}

		err := template.Must(template.New("").Parse(templateText)).Execute(&buffer, templateParams)
		if err != nil {
			return nil, err
		}

		groups[grp.Name] = string(buffer.Bytes())
	}

	return groups, nil
}

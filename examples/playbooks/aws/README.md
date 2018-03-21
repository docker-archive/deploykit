Example Playbook for AWS
========================

This playbook contains example of working with AWS.  There are commands for
starting up infrakit (which assumes you have used AWS CLI on your local computer and
the `.aws/credentials` file exists) and commands for spinning up an on-demand or
spot instance configured with Docker, Git and Go compiler.

## Adding this playbook

Adding files locally:

```
infrakit playbook add aws file://$(pwd)/index.yml
```

Adding the playbook from Github:

```
infrakit playbook add aws https://raw.githubusercontent.com/docker/infrakit/master/examples/playbooks/aws/index.yml
```

## Building an Entire VPC

It's now possible to build a whole VPC using infrakit's resource controller.
*Warning: this is not production ready.  You can't delete the resources with infrakit yet.*

1. Start up infrakit

```
$ infrakit use aws start
```

2. In another terminal, let's watch some events

```
$ infrakit local resource tail / --view 'str://{{.Type}} - {{.ID}} - {{.Message}}'
```

3. In another terminal, commit the spec to monitor resources as they are created:

```
$infrakit use aws inventory.yml | infrakit local mystack/inventory commit -y -
```

Before any resources are created, we expect to see no metadata:

```
$ infrakit local inventory/myproject keys -al
total 0:
```

4. Commit the `mystack.yml` playbook to the resource controller.  This file
has specs of all the resources and their dependencies in one place.  The
playbook also contains other commands to provision the resources individually
(eg. `infrakit use aws vpc` will provision just a vpc).

Once committed, The controller will try to reconcile and begin to provision
the resources in the VPC.  In this case it will provision these resources:

   + The VPC (equivalent to running `infrakit use aws vpc`)
   + Internet Gateway, with one route table and route to the internet through the gateway.
   The standalone equivalent: `infrakit use aws gateway` (`provision-gateway.yml) followed
   by `infrakit use aws routetable` (see `provision-routetable.yml`).
   + Two subnets (see `provision-subnet.yml`).
   + One security group (see `provision-securitygroup.yml`)

Commit the file:

```
$ $ infrakit use aws mystack | infrakit local mystack/resource commit -y -
Please enter your user name: [davidchung]:
Project? [myproject]:
CIDR block? [10.0.0.0/16]:
CIDR block? [10.0.100.0/24]:
CIDR block? [10.0.200.0/24]:
Availability Zone? [eu-central-1a]:
Availability Zone? [eu-central-1b]:
kind: resource
metadata:
  id: myproject
  name: myproject
  tags: null
options:
  ObserveInterval: 5s
properties:
  igw:
    Properties:
      AttachInternetGatewayInput:
        VpcId: '@depend(''vpc/Properties/VpcId'')@'
    select:
      Name: myproject-igw
      infrakit_created: 2018-03-18
      infrakit_scope: myproject
      infrakit_user: davidchung
    plugin: aws/ec2-internetgateway
  rtb:
    Properties:
      CreateRouteInputs:
      - DestinationCidrBlock: 0.0.0.0/0
        GatewayId: '@depend(''igw/Properties/InternetGatewayId'')@'
      CreateRouteTableInput:
        VpcId: '@depend(''vpc/Properties/VpcId'')@'
    select:
      Name: myproject-rtb
      infrakit_created: 2018-03-18
      infrakit_scope: myproject
      infrakit_user: davidchung
    plugin: aws/ec2-routetable
  sg1:
    Properties:
      AuthorizeSecurityGroupIngressInput:
      - CidrIp: 0.0.0.0/0
        FromPort: 22
        IpProtocol: tcp
        ToPort: 22
      - CidrIp: 0.0.0.0/0
        FromPort: 24864
        IpProtocol: tcp
        ToPort: 24864
      CreateSecurityGroupInput:
        Description: basic-sg
        GroupName: myproject-sg1
        VpcId: '@depend(''vpc/Properties/VpcId'')@'
    select:
      Name: myproject-sg1
      infrakit_created: 2018-03-18
      infrakit_scope: myproject
      infrakit_user: davidchung
    plugin: aws/ec2-securitygroup
  subnet1:
    Properties:
      CreateSubnetInput:
        AvailabilityZone: eu-central-1a
        CidrBlock: 10.0.100.0/24
        VpcId: '@depend(''vpc/Properties/VpcId'')@'
      RouteTableAssociation:
        RouteTableId: '@depend(''rtb/Properties/RouteTableId'')@'
    select:
      Name: myproject-subnet1
      infrakit_created: 2018-03-18
      infrakit_scope: myproject
      infrakit_user: davidchung
    plugin: aws/ec2-subnet
  subnet2:
    Properties:
      CreateSubnetInput:
        AvailabilityZone: eu-central-1b
        CidrBlock: 10.0.200.0/24
        VpcId: '@depend(''vpc/Properties/VpcId'')@'
      RouteTableAssociation:
        RouteTableId: '@depend(''rtb/Properties/RouteTableId'')@'
    select:
      Name: myproject-subnet2
      infrakit_created: 2018-03-18
      infrakit_scope: myproject
      infrakit_user: davidchung
    plugin: aws/ec2-subnet
  vpc:
    Properties:
      CreateVpcInput:
        CidrBlock: 10.0.0.0/16
      ModifyVpcAttributeInputs:
      - EnableDnsSupport:
          Value: true
      - EnableDnsHostnames:
          Value: true
    select:
      Name: myproject-vpc
      infrakit_created: 2018-03-18
      infrakit_scope: myproject
      infrakit_user: davidchung
    plugin: aws/ec2-vpc
state:
- Key: igw
  State: REQUESTED
- Key: rtb
  State: REQUESTED
- Key: sg1
  State: REQUESTED
- Key: subnet1
  State: REQUESTED
- Key: subnet2
  State: REQUESTED
- Key: vpc
  State: REQUESTED
version: ""
```

In the terminal where you are watching events, you should see:

```

CollectionUpdate - myproject/sg1 - update collection
CollectionUpdate - myproject/subnet1 - update collection
CollectionUpdate - myproject/subnet2 - update collection
CollectionUpdate - myproject/igw - update collection
CollectionUpdate - myproject/rtb - update collection
Pending - myproject/sg1 - resource blocked waiting on dependencies
Provision - myproject/vpc - provisioning resource
Pending - myproject/rtb - resource blocked waiting on dependencies
Pending - myproject/igw - resource blocked waiting on dependencies
Pending - myproject/subnet2 - resource blocked waiting on dependencies
Pending - myproject/subnet1 - resource blocked waiting on dependencies
MetadataUpdate - myproject/vpc - update metadata
Ready - myproject/vpc - resource ready
Provision - myproject/igw - provisioning resource
Provision - myproject/sg1 - provisioning resource
MetadataUpdate - myproject/igw - update metadata
Ready - myproject/igw - resource ready
Provision - myproject/rtb - provisioning resource
MetadataUpdate - myproject/sg1 - update metadata
Ready - myproject/sg1 - resource ready
Ready - myproject/rtb - resource ready
MetadataUpdate - myproject/rtb - update metadata
Provision - myproject/subnet2 - provisioning resource
Provision - myproject/subnet1 - provisioning resource
MetadataUpdate - myproject/subnet2 - update metadata
Ready - myproject/subnet2 - resource ready
MetadataUpdate - myproject/subnet2 - update metadata
MetadataUpdate - myproject/rtb - update metadata
Ready - myproject/subnet1 - resource ready
MetadataUpdate - myproject/subnet1 - update metadata
MetadataUpdate - myproject/subnet1 - update metadata
MetadataUpdate - myproject/rtb - update metadata
```

After the events stopped, you can query the inventory controller for the known
resources that's been created:

```
$ infrakit local inventory/myproject keys -al
total 289:
networking/aws/ec2-internetgateway/myproject-igw/ID
networking/aws/ec2-internetgateway/myproject-igw/LogicalID
networking/aws/ec2-internetgateway/myproject-igw/Properties/Attachments/[0]/State
networking/aws/ec2-internetgateway/myproject-igw/Properties/Attachments/[0]/VpcId
networking/aws/ec2-internetgateway/myproject-igw/Properties/InternetGatewayId
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[0]/Key
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[0]/Value
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[1]/Key
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[1]/Value
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[2]/Key
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[2]/Value
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[3]/Key
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[3]/Value
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[4]/Key
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[4]/Value
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[5]/Key
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[5]/Value
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[6]/Key
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[6]/Value
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[7]/Key
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[7]/Value
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[8]/Key
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[8]/Value
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[9]/Key
networking/aws/ec2-internetgateway/myproject-igw/Properties/Tags/[9]/Value
networking/aws/ec2-internetgateway/myproject-igw/Tags/Name
networking/aws/ec2-internetgateway/myproject-igw/Tags/infrakit_created
networking/aws/ec2-internetgateway/myproject-igw/Tags/infrakit_namespace
networking/aws/ec2-internetgateway/myproject-igw/Tags/infrakit_scope
networking/aws/ec2-internetgateway/myproject-igw/Tags/infrakit_user
networking/aws/ec2-internetgateway/myproject-igw/Tags/infrakit_collection
networking/aws/ec2-internetgateway/myproject-igw/Tags/infrakit_instance
networking/aws/ec2-internetgateway/myproject-igw/Tags/infrakit_link
networking/aws/ec2-internetgateway/myproject-igw/Tags/infrakit_link_context
networking/aws/ec2-internetgateway/myproject-igw/Tags/infrakit_link_created
networking/aws/ec2-routetable/myproject-rtb/ID
networking/aws/ec2-routetable/myproject-rtb/LogicalID
networking/aws/ec2-routetable/myproject-rtb/Properties/Associations/[0]/Main
networking/aws/ec2-routetable/myproject-rtb/Properties/Associations/[0]/RouteTableAssociationId
networking/aws/ec2-routetable/myproject-rtb/Properties/Associations/[0]/RouteTableId
networking/aws/ec2-routetable/myproject-rtb/Properties/Associations/[0]/SubnetId
networking/aws/ec2-routetable/myproject-rtb/Properties/Associations/[1]/Main
networking/aws/ec2-routetable/myproject-rtb/Properties/Associations/[1]/RouteTableAssociationId
networking/aws/ec2-routetable/myproject-rtb/Properties/Associations/[1]/RouteTableId
networking/aws/ec2-routetable/myproject-rtb/Properties/Associations/[1]/SubnetId
networking/aws/ec2-routetable/myproject-rtb/Properties/PropagatingVgws
networking/aws/ec2-routetable/myproject-rtb/Properties/RouteTableId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[0]/DestinationCidrBlock
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[0]/DestinationIpv6CidrBlock
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[0]/DestinationPrefixListId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[0]/EgressOnlyInternetGatewayId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[0]/GatewayId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[0]/InstanceId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[0]/InstanceOwnerId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[0]/NatGatewayId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[0]/NetworkInterfaceId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[0]/Origin
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[0]/State
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[0]/VpcPeeringConnectionId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[1]/DestinationCidrBlock
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[1]/DestinationIpv6CidrBlock
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[1]/DestinationPrefixListId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[1]/EgressOnlyInternetGatewayId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[1]/GatewayId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[1]/InstanceId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[1]/InstanceOwnerId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[1]/NatGatewayId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[1]/NetworkInterfaceId
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[1]/Origin
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[1]/State
networking/aws/ec2-routetable/myproject-rtb/Properties/Routes/[1]/VpcPeeringConnectionId
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[0]/Key
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[0]/Value
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[1]/Key
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[1]/Value
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[2]/Key
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[2]/Value
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[3]/Key
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[3]/Value
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[4]/Key
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[4]/Value
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[5]/Key
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[5]/Value
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[6]/Key
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[6]/Value
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[7]/Key
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[7]/Value
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[8]/Key
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[8]/Value
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[9]/Key
networking/aws/ec2-routetable/myproject-rtb/Properties/Tags/[9]/Value
networking/aws/ec2-routetable/myproject-rtb/Properties/VpcId
networking/aws/ec2-routetable/myproject-rtb/Tags/Name
networking/aws/ec2-routetable/myproject-rtb/Tags/infrakit_created
networking/aws/ec2-routetable/myproject-rtb/Tags/infrakit_namespace
networking/aws/ec2-routetable/myproject-rtb/Tags/infrakit_scope
networking/aws/ec2-routetable/myproject-rtb/Tags/infrakit_user
networking/aws/ec2-routetable/myproject-rtb/Tags/infrakit_collection
networking/aws/ec2-routetable/myproject-rtb/Tags/infrakit_instance
networking/aws/ec2-routetable/myproject-rtb/Tags/infrakit_link
networking/aws/ec2-routetable/myproject-rtb/Tags/infrakit_link_context
networking/aws/ec2-routetable/myproject-rtb/Tags/infrakit_link_created
networking/aws/ec2-securitygroup/myproject-sg1/ID
networking/aws/ec2-securitygroup/myproject-sg1/LogicalID
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Description
networking/aws/ec2-securitygroup/myproject-sg1/Properties/GroupId
networking/aws/ec2-securitygroup/myproject-sg1/Properties/GroupName
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissions/[0]/FromPort
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissions/[0]/IpProtocol
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissions/[0]/IpRanges/[0]/CidrIp
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissions/[0]/Ipv6Ranges
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissions/[0]/PrefixListIds
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissions/[0]/ToPort
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissions/[0]/UserIdGroupPairs
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissions/[1]/FromPort
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissions/[1]/IpProtocol
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissions/[1]/IpRanges/[0]/CidrIp
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissions/[1]/Ipv6Ranges
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissions/[1]/PrefixListIds
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissions/[1]/ToPort
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissions/[1]/UserIdGroupPairs
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissionsEgress/[0]/FromPort
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissionsEgress/[0]/IpProtocol
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissionsEgress/[0]/IpRanges/[0]/CidrIp
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissionsEgress/[0]/Ipv6Ranges
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissionsEgress/[0]/PrefixListIds
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissionsEgress/[0]/ToPort
networking/aws/ec2-securitygroup/myproject-sg1/Properties/IpPermissionsEgress/[0]/UserIdGroupPairs
networking/aws/ec2-securitygroup/myproject-sg1/Properties/OwnerId
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[0]/Key
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[0]/Value
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[1]/Key
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[1]/Value
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[2]/Key
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[2]/Value
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[3]/Key
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[3]/Value
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[4]/Key
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[4]/Value
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[5]/Key
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[5]/Value
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[6]/Key
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[6]/Value
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[7]/Key
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[7]/Value
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[8]/Key
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[8]/Value
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[9]/Key
networking/aws/ec2-securitygroup/myproject-sg1/Properties/Tags/[9]/Value
networking/aws/ec2-securitygroup/myproject-sg1/Properties/VpcId
networking/aws/ec2-securitygroup/myproject-sg1/Tags/Name
networking/aws/ec2-securitygroup/myproject-sg1/Tags/infrakit_created
networking/aws/ec2-securitygroup/myproject-sg1/Tags/infrakit_namespace
networking/aws/ec2-securitygroup/myproject-sg1/Tags/infrakit_scope
networking/aws/ec2-securitygroup/myproject-sg1/Tags/infrakit_user
networking/aws/ec2-securitygroup/myproject-sg1/Tags/infrakit_collection
networking/aws/ec2-securitygroup/myproject-sg1/Tags/infrakit_instance
networking/aws/ec2-securitygroup/myproject-sg1/Tags/infrakit_link
networking/aws/ec2-securitygroup/myproject-sg1/Tags/infrakit_link_context
networking/aws/ec2-securitygroup/myproject-sg1/Tags/infrakit_link_created
networking/aws/ec2-subnet/myproject-subnet1/ID
networking/aws/ec2-subnet/myproject-subnet1/LogicalID
networking/aws/ec2-subnet/myproject-subnet1/Properties/AssignIpv6AddressOnCreation
networking/aws/ec2-subnet/myproject-subnet1/Properties/AvailabilityZone
networking/aws/ec2-subnet/myproject-subnet1/Properties/AvailableIpAddressCount
networking/aws/ec2-subnet/myproject-subnet1/Properties/CidrBlock
networking/aws/ec2-subnet/myproject-subnet1/Properties/DefaultForAz
networking/aws/ec2-subnet/myproject-subnet1/Properties/Ipv6CidrBlockAssociationSet
networking/aws/ec2-subnet/myproject-subnet1/Properties/MapPublicIpOnLaunch
networking/aws/ec2-subnet/myproject-subnet1/Properties/State
networking/aws/ec2-subnet/myproject-subnet1/Properties/SubnetId
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[0]/Key
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[0]/Value
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[10]/Key
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[10]/Value
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[1]/Key
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[1]/Value
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[2]/Key
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[2]/Value
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[3]/Key
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[3]/Value
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[4]/Key
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[4]/Value
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[5]/Key
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[5]/Value
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[6]/Key
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[6]/Value
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[7]/Key
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[7]/Value
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[8]/Key
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[8]/Value
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[9]/Key
networking/aws/ec2-subnet/myproject-subnet1/Properties/Tags/[9]/Value
networking/aws/ec2-subnet/myproject-subnet1/Properties/VpcId
networking/aws/ec2-subnet/myproject-subnet1/Tags/Name
networking/aws/ec2-subnet/myproject-subnet1/Tags/infrakit_created
networking/aws/ec2-subnet/myproject-subnet1/Tags/infrakit_namespace
networking/aws/ec2-subnet/myproject-subnet1/Tags/infrakit_scope
networking/aws/ec2-subnet/myproject-subnet1/Tags/infrakit_user
networking/aws/ec2-subnet/myproject-subnet1/Tags/infrakit_collection
networking/aws/ec2-subnet/myproject-subnet1/Tags/infrakit_instance
networking/aws/ec2-subnet/myproject-subnet1/Tags/infrakit_link
networking/aws/ec2-subnet/myproject-subnet1/Tags/infrakit_link_context
networking/aws/ec2-subnet/myproject-subnet1/Tags/infrakit_link_created
networking/aws/ec2-subnet/myproject-subnet1/Tags/routeTableAssociation
networking/aws/ec2-subnet/myproject-subnet2/ID
networking/aws/ec2-subnet/myproject-subnet2/LogicalID
networking/aws/ec2-subnet/myproject-subnet2/Properties/AssignIpv6AddressOnCreation
networking/aws/ec2-subnet/myproject-subnet2/Properties/AvailabilityZone
networking/aws/ec2-subnet/myproject-subnet2/Properties/AvailableIpAddressCount
networking/aws/ec2-subnet/myproject-subnet2/Properties/CidrBlock
networking/aws/ec2-subnet/myproject-subnet2/Properties/DefaultForAz
networking/aws/ec2-subnet/myproject-subnet2/Properties/Ipv6CidrBlockAssociationSet
networking/aws/ec2-subnet/myproject-subnet2/Properties/MapPublicIpOnLaunch
networking/aws/ec2-subnet/myproject-subnet2/Properties/State
networking/aws/ec2-subnet/myproject-subnet2/Properties/SubnetId
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[0]/Key
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[0]/Value
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[10]/Key
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[10]/Value
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[1]/Key
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[1]/Value
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[2]/Key
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[2]/Value
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[3]/Key
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[3]/Value
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[4]/Key
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[4]/Value
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[5]/Key
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[5]/Value
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[6]/Key
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[6]/Value
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[7]/Key
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[7]/Value
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[8]/Key
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[8]/Value
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[9]/Key
networking/aws/ec2-subnet/myproject-subnet2/Properties/Tags/[9]/Value
networking/aws/ec2-subnet/myproject-subnet2/Properties/VpcId
networking/aws/ec2-subnet/myproject-subnet2/Tags/Name
networking/aws/ec2-subnet/myproject-subnet2/Tags/infrakit_created
networking/aws/ec2-subnet/myproject-subnet2/Tags/infrakit_namespace
networking/aws/ec2-subnet/myproject-subnet2/Tags/infrakit_scope
networking/aws/ec2-subnet/myproject-subnet2/Tags/infrakit_user
networking/aws/ec2-subnet/myproject-subnet2/Tags/infrakit_collection
networking/aws/ec2-subnet/myproject-subnet2/Tags/infrakit_instance
networking/aws/ec2-subnet/myproject-subnet2/Tags/infrakit_link
networking/aws/ec2-subnet/myproject-subnet2/Tags/infrakit_link_context
networking/aws/ec2-subnet/myproject-subnet2/Tags/infrakit_link_created
networking/aws/ec2-subnet/myproject-subnet2/Tags/routeTableAssociation
networking/aws/ec2-vpc/myproject-vpc/ID
networking/aws/ec2-vpc/myproject-vpc/LogicalID
networking/aws/ec2-vpc/myproject-vpc/Properties/CidrBlock
networking/aws/ec2-vpc/myproject-vpc/Properties/DhcpOptionsId
networking/aws/ec2-vpc/myproject-vpc/Properties/InstanceTenancy
networking/aws/ec2-vpc/myproject-vpc/Properties/Ipv6CidrBlockAssociationSet
networking/aws/ec2-vpc/myproject-vpc/Properties/IsDefault
networking/aws/ec2-vpc/myproject-vpc/Properties/State
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[0]/Key
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[0]/Value
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[1]/Key
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[1]/Value
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[2]/Key
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[2]/Value
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[3]/Key
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[3]/Value
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[4]/Key
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[4]/Value
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[5]/Key
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[5]/Value
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[6]/Key
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[6]/Value
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[7]/Key
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[7]/Value
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[8]/Key
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[8]/Value
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[9]/Key
networking/aws/ec2-vpc/myproject-vpc/Properties/Tags/[9]/Value
networking/aws/ec2-vpc/myproject-vpc/Properties/VpcId
networking/aws/ec2-vpc/myproject-vpc/Tags/Name
networking/aws/ec2-vpc/myproject-vpc/Tags/infrakit_created
networking/aws/ec2-vpc/myproject-vpc/Tags/infrakit_namespace
networking/aws/ec2-vpc/myproject-vpc/Tags/infrakit_scope
networking/aws/ec2-vpc/myproject-vpc/Tags/infrakit_user
networking/aws/ec2-vpc/myproject-vpc/Tags/infrakit_collection
networking/aws/ec2-vpc/myproject-vpc/Tags/infrakit_instance
networking/aws/ec2-vpc/myproject-vpc/Tags/infrakit_link
networking/aws/ec2-vpc/myproject-vpc/Tags/infrakit_link_context
networking/aws/ec2-vpc/myproject-vpc/Tags/infrakit_link_created
```

5. Provision a spot instance in your new VPC

There's an playbook command called `spot` which will guide you through
provisioning a single spot instance in one of the subnets.  You can pick
`subnet1` or `subnet2` in the prompt.

```
$ infrakit use aws spot
Please enter your user name: [davidchung]:
Project? [myproject]:
AMI? [ami-df8406b0]:
Instance type? [t2.micro]:
Host name? [myproject-Zm6UfgDt]:
Spot price? [0.03]:
SSH key? [infrakit]:
Subnet? [subnet2]:
Private IP address? [10.0.200.0/24]: 10.0.200.100
Security group ID? [sg-2e3f8143]:
```

This command can sometimes timeout because it takes a while to provision a spot
instance.  In this case, you can see if it's created:

```
$ infrakit local aws/ec2-spot-instance describe
ID                            	LOGICAL                       	TAGS
sir-m81rhchp                  	10.0.200.100                  	Name=myproject-Zm6UfgDt,infrakit_created=2018-03-18,infrakit_namespace=davidchung,infrakit_scope=myproject,infrakit_user=davidchung
```

or via the inventory controller.   We query for entries under the `compute` category
(see `inventory.yml` where we defined the `compute` category) and under the
`aws/ec2-spot-instance` (the plugin name as sub namespace):

```
$ infrakit local inventory/myproject keys compute/aws/ec2-spot-instance
myproject-Zm6UfgDt

$infrakit local inventory/myproject keys compute/aws/ec2-spot-instance/myproject-Zm6UfgDt/Properties/Instance

# A bunch of fields...

$ infrakit local inventory/myproject cat compute/aws/ec2-spot-instance/myproject-Zm6UfgDt/Properties/Instance/PublicIpAddress
18.196.88.253
```

Let's try to ssh in:

```
$ ssh ubuntu@$(infrakit local inventory/myproject cat compute/aws/ec2-spot-instance/myproject-Zm6UfgDt/Properties/Instance/PublicIpAddress)
Welcome to Ubuntu 16.04.3 LTS (GNU/Linux 4.4.0-1041-aws x86_64)

 * Documentation:  https://help.ubuntu.com
 * Management:     https://landscape.canonical.com
 * Support:        https://ubuntu.com/advantage

  Get cloud support with Ubuntu Advantage Cloud Guest:
    http://www.ubuntu.com/business/services/cloud

107 packages can be updated.
48 updates are security updates.


*** System restart required ***
Last login: Mon Mar 19 00:20:36 2018 from 97.105.231.235
ubuntu@ip-10-0-200-100:~$
```

## Clean up

*Currently we do not support termination of resources. So you must do this manually.*

The ordering to destroy:


1. Destroy the instances first
```
$ infrakit local aws/ec2-spot-instance destroy ...
```

2. Destroy the security groups:
```
$ infrakit local aws/ec2-securitygroup destroy ...
```

3. Destroy the subnets
```
$ infrakit local aws/ec2-subnet destroy ...
```

4. Destroy the route tables
```
$ infrakit local aws/ec2-routetable destroy ...
```

5. Destroy the gateway
```
$ infrakit local aws/ec2-internetgateway destroy ...
```

6. Destroy the VPC
```
$ infrakit local aws/ec2-vpc destroy ...
```
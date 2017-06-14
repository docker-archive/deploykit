# API types and common fields

# Allocation
The method and details for allocating a group.  Groups are homogeneous (cattle) when the `Size` field is set, or
nearly-homogeneous (pets) when the `LogicalIDs` field is set.

Groups using `LogicalIDs` will converge towards maintaining a number of Instances matching the number of `LogicalIDs`,
where each Instance is uniquely assigned a logical ID.  The ID will be present in the `LogicalID` field of the Instance
[Description](#instance-description) and [Spec](#instance-spec).  Logical IDs are useful for behavior such as managing
a pool of `Attachments` for state attached to instances.

Fields:
- `Size`: An integer, the number of instances to maintain in the Group.
- `LogicalIDs`: An array of strings, the logical identifeirs to maintain in the group.

# Index
Index is a context object that is used to denote the instance's relationship with respect to the group it belongs.
An Index has two fields: a group ID and a sequence number.  The group ID is the identifier of the group, while the
sequence number represents the order in which the instance is created with respect to the group.  So for a group of
size N, the sequence number will be 0, 1, 2, ... N-1.  This can be used by the flavor plugin to determine which one
of the instances, as represented by a fixed set of nodes (e.g. pets, or stateful nodes), the current instance is.
In cases where provisioning instances where IP addresses cannot be assigned upfront, this provides a mechanism for
looking up which IP address a particular node represents if given a list of IP addresses as a result of instance
creations for this group.  A similar concept can be found in AWS Autoscaling group's Launch index.


# Group ID
A globally-unique string identifier for an Group.


# Group Description

Fields:
- `Instances`: An array of [Instance Descriptions](#instance-description)
- `Converged`: `true` if the state of the Group matches the most recently
  [Committed](group.md#method-group-commit-group) state, `false` otherwise.


# Group Spec
The declared specification of a Group.

Fields:
- `ID`: [Group ID](types.md#group-id)
- `Properties`: A JSON object representing the Group.  The schema is defined by the Group plugin in use.

### Instance Attachment
An external entity associated with an Instance.  The meaning of an Attachment is defined by the Instance plugin.

Fields:
- `ID`: A unique identifier for the Attachment
- `Type`: An identifier for the kind of attachment.


### Instance Description
The description of an existing Instance.

Fields:
- `ID`: An [Instance ID](types.md#instance-id)
- `LogicalID`: [Logical ID](#logical-id)
- `Tags`: [Instance Tags](#instance-tags)

### Instance ID
A globally-unique string identifier for an Instance.

### Context
An operation context. Contains a reason code that can be either
  + rolling_update
  + termination
It is currently used to provide context for proper handling of instance.Destroy

### Instance Spec
The declared specification of an Instance.

Fields:
- `Properties`: A JSON object representing the Instance to create.  The schema is defined by the Instance plugin in use.
- `Tags`: [Instance Tags](#instance-tags)
- `Init`: a shell script to run when the instance boots.
- `LogicalID`: [Logical ID](#logical-id)
- `Attachments`: an array [Attachments](#instance-attachment)


### Instance Tags
A key (string) to value (string) mapping of attributes to include on an instance.  Tags are useful for user-defined
Instance grouping and metadata.


### Logical ID
A possibly-reused logical identifier for an Instance.

### Labels
A map or dictionary of key-value pairs that label / tag the instance.  Same as [Instance Tags](#instance-tags)
# Group plugin API

<!-- SOURCE-CHECKSUM pkg/spi/group/* 70857e1ae1e930df87b79433ef30be20568e30752d3b7214f4deafc12aa2cedc7ff3d7d01e46e2cd72f7eeb23400d0ab8affff2bd04b9f3cf016e2ac -->

## API

### Method `Group.CommitGroup`
Submits the configuration for a group.  The Group plugin is responsible for making any changes necessary to effect the
configuration.

#### Request
```json
{
  "Spec": {
    "ID": "group_id",
    "Properties": {}
  },
  "Pretend": true
}
```

Parameters:
- `Spec`: A [Group Spec](types.md#group-spec)
- `Pretend`: Whether to actually perform the change.  If `false`, the request will have no side-effects.

#### Response
```json
{
  "Details": "human readable text"
}
```

Fields:
- `Details`: A human-readable description of the commit action, or proposed action if `Pretend` was `true`.


### Method `Group.FreeGroup`
Removes a Group from active management.  This operation is non-destructive - it will not destroy or modify any resources
associated with the Group.  However, the Plugin will no longer attempt to maintain the state of the Group.

#### Request
```json
{
  "ID": "group_id"
}
```

Parameters:
- `ID`: [Group ID](types.md#group-id)

#### Response
```json
{
  "OK": true
}
```

Fields:
- `OK`: Whether the operation succeeded.

### Method `Group.DescribeGroup`
Fetches details about the current status of a Group.

#### Request
```json
{
  "ID": "group_Id"
}
```

Parameters:
- `ID`: [Group ID](types.md#group-id)

#### Response
```json
{
  "Description": {
    "Instances": [
      {
        "ID": "instance_id",
        "LogicalID": "logical_id",
        "Tags": {
          "tag_key": "tag_value"
        }
      }
    ],
    "Converged": true
  }
}
```

Fields:
- `Description`: A [Group Description](types.md#group-description)

### Method `Group.DestroyGroup`

#### Request
```json
{
  "ID": "group_Id"
}
```

Parameters:
- `ID`: [Group ID](types.md#group-id)

#### Response
```json
{
  "OK": true
}
```

Fields:
- `OK`: Whether the operation succeeded.

### Method `Group.InspectGroups`
Fetches details about the state associated with a Group.

#### Request
```json
{}
```

Parameters: None

#### Response
```json
{
  "Groups": [
    {
      "ID": "group_id",
      "Properties": {}
    }
  ]
}
```

Fields:
- `Groups`: An array of [Group Specs](types.md#group-spec)

### Method `Group.DestroyInstances`
Destroy instances identified from the given group.  Returns error if any of the given
are not found in the group or if the destroy fails.

#### Request
```json
{
  "ID" : "group_id",
  "Instances" : [
     "instance-id1",
     "instance-id2",
     "instance-id3"
  ]
}
```

Parameters: None

Fields:
- `ID`: The group id.
- `Instances` : An array of Instance IDs

#### Response
```json
{
  "ID": "group_id"
}
```

### Method `Group.Size`
Returns the desired / target size of the group.  This may not match the size of
the list from DescribeGroup()

#### Request
```json
{
  "ID" : "group_id"
}
```

Parameters: None

Fields:
- `ID`: The group id.

#### Response
```json
{
  "ID": "group_id"
  "Size": 100
}
```

### Method `Group.SetSize`
Sets the desired / target size of the group.  This is the same as editing the config
and call commit.

#### Request
```json
{
  "ID" : "group_id",
  "Size" : 100
}
```

Parameters: None

Fields:
- `ID`: The group id.
- `Size`: The group target size.

#### Response
```json
{
  "ID" : "group_id"
}
```

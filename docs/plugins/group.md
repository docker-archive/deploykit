# Group plugin API

<!-- SOURCE-CHECKSUM pkg/spi/group/* 4bc86b2ae0893db92f880ab4bb2479b5def55746 -->

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

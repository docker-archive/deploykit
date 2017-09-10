# Instance plugin API

<!-- SOURCE-CHECKSUM pkg/spi/instance/* 6b3c98bed4470312a41376f651cee99a9e35ffb02e5726e3feca6e60faf1d3ea6e9ae1d8eab96f9ea799fded295ca3ba84a09455f04a579ac4123eb89117db2da2ea1073a6f94b241e6c6a9a0a9048d452c44631323150dfeffdfde2b8c5ba85e274fabe -->

## API

### Method `Instance.Validate`
Checks whether an instance configuration is valid.  Must be free of side-effects.

#### Request
```json
{
  "Properties": {}
}
```

Parameters:
- `Properties`: A JSON object representing the Instance.  The schema is defined by the Instance plugin in use.


#### Response
```json
{
  "OK": true
}
```

Fields:
- `OK`: Whether the operation succeeded.

### Method `Instance.Provision`
Provisions a new instance.

#### Request
```json
{
  "Spec": {
    "Properties": {},
    "Tags": {"tag_key": "tag_value"},
    "Init": "",
    "LogicalID": "logical_id",
    "Attachments": [{"ID": "attachment_id", "Type": "block-device"}]
  }
}
```

Parameters:
- `Spec`: an [Instance Spec](types.md#instance-spec).

#### Response
```json
{
  "ID": "instance_id"
}
```

Fields:
- `ID`: An [instance ID](types.md#instance-id).

### Method `Instance.Destroy`
Permanently destroys an instance.

#### Request
```json
{
  "Instance": "instance_id",
  "Context" : {
    "Reason" : "rolling_update"
  }
}
```

Parameters:
- `Instance`: An [instance ID](types.md#instance-id).
- `Context` : An [instance Context](types.md#context).

#### Response
```json
{
  "OK": true
}
```

Fields:
- `OK`: Whether the operation succeeded.

### Method `Instance.Label`
Labels an instance.  The plugin should add or update the labels given.

#### Request
```json
{
  "Instance": "instance_id",
  "Labels" : {
     "label1" : "value1",
     "label2" : "value2",
     "label3" : "value3"
  }
}
```

Parameters:
- `Instance`: An [instance ID](types.md#instance-id).
- `Labels`: A [map](types.md#instance-tags) of labels or instance tags.

#### Response
```json
{
  "OK": true
}
```

Fields:
- `OK`: Whether the operation succeeded.

### Method `Instance.DescribeInstances`
Fetches details about Instances.

#### Request
```json
{
  "Tags": {"tag_key": "tag_value"},
  "Properties" : true
}
```

Parameters:
- `Tags`: Instance tags to match.  If multiple tags are specified, only Instances matching all tags are returned.
- `Properties`: Boolean to indicate whether the client requests additional details via the `Description.Properties` field.
#### Response
```json
{
  "Descriptions": [
    {
      "ID": "instance_id",
      "LogicalID": "logical_id",
      "Tags": {"tag_key": "tag_value"},
      "Properties" : { "some_status" : "ok", "some_state" : 10 }
    }
  ]
}
```

Fields:
- `Descriptions`: An array of matching [Instance Descriptions](types.md#instance-description)

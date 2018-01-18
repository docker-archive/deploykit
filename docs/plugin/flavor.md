# Flavor plugin API

<!-- SOURCE-CHECKSUM pkg/spi/flavor/* 3994956c9645940bda14eff43681ce48c7f24095 -->

## API

### Method `Flavor.Validate`
Checks whether a Flavor configuration is valid.

#### Request
```json
{
  "Properties": {},
  "Allocation": {
    "Size": 1,
    "LogicalIDs": ["logical_id_1"]
  }
}
```

Parameters:
- `Properties`: A JSON object representing the Flavor.  The schema is defined by the Flavor plugin in use.
- `Allocation`: An [Allocation](types.md#allocation)


#### Response
```json
{
  "OK": true
}
```

`OK`: Whether the operation succeeded.

### Method `Flavor.Prepare`
Instructs the Flavor Plugin to prepare an Instance with any additional configuration it wishes to include.  The Flavor
may add, remove, or modify any fields in the Instance `Spec` by returning the desired Instance configuration.

#### Request
```json
{
  "Properties": {},
  "Spec": {
    "Properties": {},
    "Tags": {
      "tag_key": "tag_value"
      },
    "Init": "init script",
    "LogicalID": "logical_id",
    "Attachments": ["attachment_id"]
  },
  "Allocation": {
    "Size": 1,
    "LogicalIDs": ["logical_id_1"]
  },
  "Index" : {
    "Group" : "workers",
    "Sequence" : 100
  }
}
```

Parameters:
- `Properties`: A JSON object representing the Flavor.  The schema is defined by the Flavor plugin in use.
- `Spec`: The [Spec](types.md#instance-spec) of the Instance being prepared.
- `Allocation`: An [Allocation](types.md#allocation)
- `Index`: an [Index](types.md#index)

#### Response
```json
{
  "Spec": {
    "Properties": {},
    "Tags": {
      "tag_key": "tag_value"
      },
    "Init": "init script",
    "LogicalID": "logical_id",
    "Attachments": ["attachment_id"]
  }
}
```

Fields:
- `Spec`: The [Spec](types.md#instance-spec) of the Instance, with the Flavor's adjustments made.

### Method `Flavor.Healthy`
Checks whether the Flavor plugin considers an Instance to be healthy.

#### Request
```json
{
  "Properties": {},
  "Instance": {
    "ID": "instance_id",
    "LogicalID": "logical_id",
    "Tags": {
      "tag_key": "tag_value"
    }
  }
}
```

Parameters:
- `Properties`: A JSON object representing the Flavor.  The schema is defined by the Flavor plugin in use.
- `Instance`: The [Instance Description](types.md#instance-description)

#### Response
```json
{
  "Health": 1
}
```

Fields:
- `Health`: An integer representing the health the instance. `0` for 'unknown', `1` for 'healthy', or `2' for
  'unhealthy'

### Method `Flavor.Drain`
Informs the Flavor plugin that an Instance will soon be terminated, and allows the plugin to perform any necessary
cleanup work prior to removing the instance.

#### Request
```json
{
  "Properties": {},
  "Instance": {
    "ID": "instance_id",
    "LogicalID": "logical_id",
    "Tags": {
      "tag_key": "tag_value"
    }
  }
}
```

Parameters:
- `Properties`: A JSON object representing the Flavor.  The schema is defined by the Flavor plugin in use.
- `Instance`: The [Instance Description](types.md#instance-description)

#### Response
```json
{
  "OK": true
}
```

Fields:
- `OK`: Whether the operation succeeded.

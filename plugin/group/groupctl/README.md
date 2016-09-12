## groupctl

This is a dummy application to demonstrate the default Group plugin behavior.  After starting it up, you can create
a group with the following commands:

```bash
# Start the plugin
$ go run main.go run

# In another terminal

$ cat << EOF > group.properties
{
  "ID": "workers",
  "Properties": {
    "Size": 1,
    "InstancePlugin": "aws",
    "InstancePluginProperties": {
      "Region": "us-west-2",
      "Cluster": "bill-testing",
      "Instance": {
        "group": "workers",
        "run_instances_input": {
          "ImageId": "ami-f701cb97",
          "InstanceType": "t2.micro",
          "KeyName": "bill-laptop"
        }
      }
    }
  }
}
EOF

# Start watching
$ curl -X POST localhost:8888/Watch --data @group.properties

# Describe a potential update (after editing group.properties)
$ curl -X POST localhost:8888/DescribeUpdate --data @group.properties

# Perform an update
$ curl -X POST localhost:8888/UpdateGroup --data @group.properties

# Destroy
$ curl -X POST localhost:8888/DestroyGroup/workers
```

package mock

//go:generate mockgen -package instance -destination spi/instance/instance.go github.com/docker/infrakit/spi/instance Plugin
//go:generate mockgen -package client -destination docker/docker/client/api.go github.com/docker/docker/client APIClient
//go:generate mockgen -package group -destination plugin/group/group.go github.com/docker/infrakit/plugin/group Scaled

//go:generate mockgen -package mock -destination aws/ec2/mock_ec2iface.go github.com/aws/aws-sdk-go/service/ec2/ec2iface EC2API

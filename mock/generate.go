package mock

//go:generate mockgen -package instance -destination spi/instance/instance.go github.com/docker/libmachete/spi/instance Plugin
//go:generate mockgen -package mock -destination ec2/mock_ec2iface.go github.com/aws/aws-sdk-go/service/ec2/ec2iface EC2API
//go:generate mockgen -package client -destination docker/engine-api/client/api.go github.com/docker/engine-api/client APIClient
//go:generate mockgen -package updater -destination plugin/group/updater/updater.go github.com/docker/libmachete/plugin/group/util Executor

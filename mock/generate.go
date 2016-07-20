package mock

//go:generate mockgen -package instance -destination spi/instance/instance.go github.com/docker/libmachete/spi/instance Provisioner
//go:generate mockgen -package mock -destination ec2/mock_ec2iface.go github.com/aws/aws-sdk-go/service/ec2/ec2iface EC2API

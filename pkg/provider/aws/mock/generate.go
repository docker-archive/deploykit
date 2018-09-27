package mock // import "github.com/docker/infrakit/pkg/provider/aws/mock"

//go:generate mockgen -package mock // import "github.com/docker/infrakit/pkg/provider/aws/mock" -destination ec2/mock_ec2iface.go github.com/aws/aws-sdk-go/service/ec2/ec2iface EC2API

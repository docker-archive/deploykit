package scaler

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/docker/libmachete/client"
	mock_ssh_util "github.com/docker/libmachete/mock/spi/util/sshutil"
	aws_provider "github.com/docker/libmachete/provider/aws"
	"github.com/docker/libmachete/server"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/util/sshutil"
	"github.com/drewolson/testflight"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"strconv"
	"sync"
	"testing"
	"time"
)

// fakeEc2 is a partial implementation of EC2API that pretends to run and terminate instances.
type fakeEc2 struct {
	ec2iface.EC2API

	nextID      int
	instanceIds []string
	lock        sync.Mutex
}

func (e *fakeEc2) CreateTags(*ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	// No-op
	return nil, nil
}

func (e *fakeEc2) DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	instances := []*ec2.Instance{}
	for _, id := range e.instanceIds {
		copy := id
		instances = append(instances, &ec2.Instance{InstanceId: &copy, KeyName: aws.String("key")})
	}

	return &ec2.DescribeInstancesOutput{Reservations: []*ec2.Reservation{{Instances: instances}}}, nil
}

func (e *fakeEc2) RunInstances(*ec2.RunInstancesInput) (*ec2.Reservation, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	e.nextID++
	id := strconv.Itoa(e.nextID)
	e.instanceIds = append(e.instanceIds, id)
	return &ec2.Reservation{Instances: []*ec2.Instance{{InstanceId: &id}}}, nil
}

func (e *fakeEc2) TerminateInstances(input *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	id := *input.InstanceIds[0]
	position := -1
	for i := range e.instanceIds {
		if e.instanceIds[i] == id {
			position = i
			break
		}
	}
	if position == -1 {
		return nil, errors.New("Instance nt found")
	}

	e.instanceIds = append(e.instanceIds[:position], e.instanceIds[position+1:]...)
	return &ec2.TerminateInstancesOutput{TerminatingInstances: []*ec2.InstanceStateChange{{}}}, nil
}

func (e *fakeEc2) CreateKeyPair(*ec2.CreateKeyPairInput) (*ec2.CreateKeyPairOutput, error) {
	return &ec2.CreateKeyPairOutput{KeyMaterial: aws.String("fake key data")}, nil
}

func (e *fakeEc2) DeleteKeyPair(*ec2.DeleteKeyPairInput) (*ec2.DeleteKeyPairOutput, error) {
	return &ec2.DeleteKeyPairOutput{}, nil
}

func (e *fakeEc2) maybeResetIds(newIds []string, predicate func([]string) bool) bool {
	e.lock.Lock()
	defer e.lock.Unlock()

	if predicate(e.instanceIds) {
		e.instanceIds = newIds
		return true
	}

	return false
}

func resetAtTarget(backend *fakeEc2, target int, newIds []string) {
	metTargetLength := func(currentIds []string) bool {
		return len(currentIds) == target
	}

	for {
		if backend.maybeResetIds(newIds, metTargetLength) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestScalerIntegration combines a scaler, HTTP client, machete server, and provisioner backend to ensure all components
// work together.
func TestScalerIntegration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	backend := &fakeEc2{}
	runnerMock := mock_ssh_util.NewMockCommandRunner(ctrl)

	provisioner := aws_provider.Provisioner{
		Client:        backend,
		Cluster:       spi.ClusterID("test-cluster"),
		CommandRunner: runnerMock,
		KeyStore:      sshutil.FileSystemKeyStore(afero.NewMemMapFs(), "/"),
	}

	testflight.WithServer(server.NewHandler(provisioner), func(r *testflight.Requester) {
		target := 3
		group := "integration-test-manager"
		watcher, err := NewFixedScaler(
			10*time.Millisecond,
			client.NewInstanceProvisioner(r.Url("")),
			fmt.Sprintf(`{"Group": "%s"}`, group),
			uint(target))
		require.NoError(t, err)

		go watcher.Run()

		// Simulate course corrections needed by the scaler.
		resetAtTarget(backend, target, []string{"a"})
		resetAtTarget(backend, target, []string{"a", "b", "c", "d", "e", "f", "g"})
		resetAtTarget(backend, target, []string{"a", "b", "c"})
		watcher.Stop()
	})
}

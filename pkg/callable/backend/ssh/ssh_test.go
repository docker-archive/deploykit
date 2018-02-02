package ssh

import (
	"fmt"
	"os"
	"testing"
	"time"

	testutil "github.com/docker/infrakit/pkg/testing"
	"github.com/docker/infrakit/pkg/util/exec"
	"github.com/docker/infrakit/pkg/util/ssh"
	"github.com/stretchr/testify/require"
)

var (
	sshContainerPasswordAuth = exec.Command(`
docker run -d --rm -p {{ .port }}:22 --name {{ .container }} sickp/alpine-sshd:7.5
`)
)

func stopContainer(name string) error {
	stopContainer := exec.Command(`docker stop {{ arg 1 }}`)
	return stopContainer.Start(name)
}

func TestExecScript(t *testing.T) {

	if testutil.SkipTests("ssh") {
		t.SkipNow()
	}

	localIP := "localhost"
	user := "root"
	password := "root"
	port := ssh.RandPort(4000, 5000)
	containerName := fmt.Sprintf("ssh-test-%v", port)

	t.Log("running", containerName, "on", port)

	err := sshContainerPasswordAuth.WithContext(map[interface{}]interface{}{
		"container": containerName,
		"port":      port,
	}).Start()
	require.NoError(t, err)

	conn := ssh.Conn{
		Remote: ssh.HostPort(fmt.Sprintf("%v:%v", localIP, port)),
		Config: ssh.UsernamePasswordConfig(user, password),
	}

	var impl exec.Interface

	timeout := time.After(30 * time.Second)
loop_connect:
	for {
		select {
		case <-timeout:
			t.Fail()
			break
		case <-time.After(1 * time.Second):

			t.Log("connecting")
			impl, err = conn.Exec()
			if err == nil {
				t.Log("connected")
				break loop_connect
			} else {
				t.Log("error", err)
			}
		}
	}

	err = execScript(impl, "ls -al /bin", nil, os.Stdout)
	require.NoError(t, err)

	err = stopContainer(containerName)
	require.NoError(t, err)
}

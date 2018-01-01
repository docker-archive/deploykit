package ssh

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	testutil "github.com/docker/infrakit/pkg/testing"
	"github.com/docker/infrakit/pkg/util/exec"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

var (
	sshContainerPasswordAuth = exec.Command(`
docker run -d --rm -p {{ .port }}:22 --name {{ .container }} sickp/alpine-sshd:7.5
`)

	sshContainerKeyAuth = exec.Command(`
docker run -d --rm -p {{ .port }}:22 --name {{ .container }} \
 -v {{.pubkey }}:/root/.ssh/authorized_keys \
 -v {{.dir }}:/root/ \
sickp/alpine-sshd:7.5
`)
)

func stopContainer(name string) error {
	stopContainer := exec.Command(`docker stop {{ arg 1 }}`)
	return stopContainer.Start(name)
}

func TestPasswordAuth(t *testing.T) {

	if testutil.SkipTests("ssh") {
		t.SkipNow()
	}

	localIP := "localhost"
	user := "root"
	password := "root"
	port := RandPort(4000, 5000)
	containerName := fmt.Sprintf("ssh-test-%v", port)

	t.Log("running", containerName, "on", port)

	err := sshContainerPasswordAuth.WithContext(map[interface{}]interface{}{
		"container": containerName,
		"port":      port,
	}).Start()
	require.NoError(t, err)

	conn := Conn{
		Remote: HostPort(fmt.Sprintf("%v:%v", localIP, port)),
		Config: &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				ssh.Password(password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		},
	}

	timeout := time.After(30 * time.Second)
loop_connect:
	for {
		select {
		case <-timeout:
			t.Fail()
			break
		case <-time.After(1 * time.Second):

			t.Log("connecting")
			err := conn.connect()
			if err == nil {
				t.Log("connected")
				break loop_connect
			} else {
				t.Log("error", err)
			}
		}
	}
	t.Log("stopping sshd")
	err = stopContainer(containerName)
	require.NoError(t, err)
}

func TestKeyAuth(t *testing.T) {

	if testutil.SkipTests("ssh") {
		t.SkipNow()
	}

	dir, err := os.Getwd()
	require.NoError(t, err)

	// outdir is where the file written by scp can be found
	outdir, err := ioutil.TempDir(dir, "")
	require.NoError(t, err)
	defer os.RemoveAll(outdir)

	localIP := "localhost"
	port := RandPort(4000, 5000)
	containerName := fmt.Sprintf("ssh-test-%v", port)
	user := "root"

	t.Log("running", containerName, "on", port)

	err = sshContainerKeyAuth.WithContext(map[interface{}]interface{}{
		"container": containerName,
		"port":      port,
		"pubkey":    dir + "/testkey.pub",
		"dir":       outdir,
	}).Start()
	require.NoError(t, err)

	conn := Conn{
		Remote: HostPort(fmt.Sprintf("%v:%v", localIP, port)),
		Config: &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				PublicKeyFile(dir + "/testkey"),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		},
	}

	timeout := time.After(30 * time.Second)
loop_connect:
	for {
		select {
		case <-timeout:
			t.Fail()
			break
		case <-time.After(1 * time.Second):

			t.Log("connecting")
			err := conn.connect()
			if err == nil {
				t.Log("connected via key auth")
				break loop_connect
			} else {
				t.Log("error", err)
			}
		}
	}

	t.Log("scp file")
	dest := fmt.Sprintf("/root/test-script-%v", port)
	payload := `
#!/bin/bash
echo hello world
`
	err = conn.WriteFile(dest, bytes.NewBufferString(payload), 0666)
	require.NoError(t, err)

	// now read the file back since it's bind mounted to the current dir
	buff, err := ioutil.ReadFile(outdir + "/" + fmt.Sprintf("test-script-%v", port))
	require.NoError(t, err)
	require.Equal(t, payload, string(buff))

	t.Log("stopping sshd")
	err = stopContainer(containerName)
	require.NoError(t, err)
}

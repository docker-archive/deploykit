package tasks

import (
	"github.com/docker/libmachete/api"
	"github.com/docker/libmachete/machines"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/docker/libmachete/storage/filestore"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSSHKeyGenAndRemove(t *testing.T) {
	sshKeys := machines.NewSSHKeys(filestore.NewFileStore(afero.NewMemMapFs(), "/"))

	host := "host-name"

	events := make(chan interface{})
	err := SSHKeyGen{Keys: sshKeys}.Run(
		api.MachineRecord{MachineSummary: api.MachineSummary{MachineName: api.MachineID(host)}},
		&spi.BaseMachineRequest{},
		events)
	require.NoError(t, err)

	// Key should have been created.
	data, err := sshKeys.GetEncodedPublicKey(api.SSHKeyID(host))
	require.NoError(t, err)
	require.NotEmpty(t, data)

	err = SSHKeyRemove{Keys: sshKeys}.Run(
		api.MachineRecord{MachineSummary: api.MachineSummary{MachineName: api.MachineID(host)}},
		&spi.BaseMachineRequest{},
		events)
	require.NoError(t, err)

	// Key should have been removed.
	_, err = sshKeys.GetEncodedPublicKey(api.SSHKeyID(host))
	require.Error(t, err)
}

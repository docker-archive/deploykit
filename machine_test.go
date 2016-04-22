package libmachete

import (
	"errors"
	"github.com/docker/libmachete/mock"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

const (
	provisionerName = "provisioner"
	templateName    = "templateName"
)

var (
	provisionerParams = map[string]string{"secret": "42"}
	templateData      = []byte(`name: larry
zone: b
arch: x86_64
network:
  public: false
  interface: eth0
disks:
  - sda1
  - sda2
labels:
  a: b
  c: d`)
	overlayData = []byte(`name: steve
network:
  turbo: true`)
	unmappableOverlayData = []byte(`gpu: true`)
)

type network struct {
	Public bool
	Iface  string `yaml:"interface"`
	Turbo  bool
}

type testSchema struct {
	Name    string
	Zone    string
	Arch    string
	Network network
	Disks   []string
	Labels  map[string]string
}

func (t *testSchema) GetName() string {
	return t.Name
}

func newRegistry(t *testing.T, ctrl *gomock.Controller) (*mock.MockProvisioner, *Registry) {
	provisioner := mock.NewMockProvisioner(ctrl)
	provisioner.EXPECT().NewRequestInstance().Return(new(testSchema)).AnyTimes()

	registry := newEmptyRegistry()
	registry.Register(provisionerName, func(params map[string]string) api.Provisioner {
		require.Equal(t, provisionerParams, params)
		return provisioner
	})
	return provisioner, registry
}

func createMachine(machine *machine, overlayYaml []byte) (<-chan api.CreateInstanceEvent, error) {
	return machine.CreateMachine(
		provisionerName,
		provisionerParams,
		templateName,
		overlayYaml)
}

func newMachine(
	t *testing.T,
	ctrl *gomock.Controller,
	templateData []byte) (*mock.MockProvisioner, *machine) {

	provisioner, registry := newRegistry(t, ctrl)

	machine := &machine{
		registry: registry,
		templateLoader: func(provisioner string, name string) ([]byte, error) {
			require.Equal(t, provisionerName, provisioner)
			require.Equal(t, templateName, name)
			return templateData, nil
		}}

	return provisioner, machine
}

func TestCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner, machine := newMachine(t, ctrl, templateData)

	expectedRequest := testSchema{
		Name: "steve",
		Zone: "b",
		Arch: "x86_64",
		Network: network{
			Public: false,
			Iface:  "eth0",
			Turbo:  true},
		Disks:  []string{"sda1", "sda2"},
		Labels: map[string]string{"a": "b", "c": "d"}}
	createEvents := make(<-chan api.CreateInstanceEvent)
	provisioner.EXPECT().CreateInstance(&expectedRequest).Return(createEvents, nil)

	events, err := createMachine(machine, overlayData)
	require.Nil(t, err)

	require.Exactly(t, createEvents, events)
}

func TestCreateInvalidTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, machine := newMachine(t, ctrl, []byte("not yaml"))

	events, err := createMachine(machine, overlayData)
	require.Nil(t, events)
	require.NotNil(t, err)
}

func TestCreateInvalidOverlay(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, machine := newMachine(t, ctrl, templateData)

	events, err := createMachine(machine, []byte("not yaml"))
	require.Nil(t, events)
	require.NotNil(t, err)
}

func TestCreateExtraYamlFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner, machine := newMachine(t, ctrl, templateData)

	// TODO(wfarner): Note that this is undesirable behavior.  YAML that does not match up
	// with the schema should be rejected with an error.  See the following issue for
	// background/updates:
	// https://github.com/go-yaml/yaml/issues/136
	expectedRequest := testSchema{
		Name: "larry",
		Zone: "b",
		Arch: "x86_64",
		Network: network{
			Public: false,
			Iface:  "eth0"},
		Disks:  []string{"sda1", "sda2"},
		Labels: map[string]string{"a": "b", "c": "d"}}
	createEvents := make(<-chan api.CreateInstanceEvent)
	provisioner.EXPECT().CreateInstance(&expectedRequest).Return(createEvents, nil)

	createMachine(machine, unmappableOverlayData)
}

func TestTemplateLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, registry := newRegistry(t, ctrl)

	machine := &machine{
		registry: registry,
		templateLoader: func(provisioner string, name string) ([]byte, error) {
			return nil, errors.New("Template not found")
		}}

	events, err := createMachine(machine, overlayData)
	require.Nil(t, events)
	require.NotNil(t, err)
}

func TestUnknownProvisiner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, machine := newMachine(t, ctrl, templateData)
	events, err := machine.CreateMachine(
		"unknown provisioner",
		provisionerParams,
		templateName,
		overlayData)

	require.Nil(t, events)
	require.NotNil(t, err)
}

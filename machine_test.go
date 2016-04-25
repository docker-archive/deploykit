package libmachete

import (
	"errors"
	mock_templates "github.com/docker/libmachete/mock"
	"github.com/docker/libmachete/provisioners"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/provisioners/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

//go:generate mockgen -package mock -destination mock/mock_templates.go github.com/docker/libmachete Templates

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
	MachineName string `yaml:"name"`
	Zone        string
	Arch        string
	Network     network
	Disks       []string
	Labels      map[string]string
}

func (t *testSchema) Name() string {
	return t.MachineName
}

func newRegistry(
	t *testing.T,
	ctrl *gomock.Controller) (*mock.MockProvisioner, *provisioners.Registry) {

	provisioner := mock.NewMockProvisioner(ctrl)
	provisioner.EXPECT().NewRequestInstance().Return(new(testSchema)).AnyTimes()

	creator := mock.NewMockProvisionerBuilder(ctrl)
	creator.EXPECT().Build(gomock.Any()).Return(provisioner, nil).AnyTimes()

	registry := provisioners.NewRegistry(
		map[string]provisioners.ProvisionerBuilder{provisionerName: creator})
	return provisioner, registry
}

func create(machine *machine, overlayYaml []byte) (<-chan api.CreateInstanceEvent, error) {
	return machine.Create(
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

	templates := mock_templates.NewMockTemplates(ctrl)
	templates.EXPECT().Read(provisionerName, templateName).AnyTimes().Return(templateData, nil)

	return provisioner, &machine{registry: registry, templateLoader: templates}
}

func TestCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner, machine := newMachine(t, ctrl, templateData)

	expectedRequest := testSchema{
		MachineName: "steve",
		Zone:        "b",
		Arch:        "x86_64",
		Network: network{
			Public: false,
			Iface:  "eth0",
			Turbo:  true},
		Disks:  []string{"sda1", "sda2"},
		Labels: map[string]string{"a": "b", "c": "d"}}
	createEvents := make(<-chan api.CreateInstanceEvent)
	provisioner.EXPECT().CreateInstance(&expectedRequest).Return(createEvents, nil)

	events, err := create(machine, overlayData)
	require.Nil(t, err)

	require.Exactly(t, createEvents, events)
}

func TestCreateInvalidTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, machine := newMachine(t, ctrl, []byte("not yaml"))

	events, err := create(machine, overlayData)
	require.Nil(t, events)
	require.NotNil(t, err)
}

func TestCreateInvalidOverlay(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, machine := newMachine(t, ctrl, templateData)

	events, err := create(machine, []byte("not yaml"))
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
		MachineName: "larry",
		Zone:        "b",
		Arch:        "x86_64",
		Network: network{
			Public: false,
			Iface:  "eth0"},
		Disks:  []string{"sda1", "sda2"},
		Labels: map[string]string{"a": "b", "c": "d"}}
	createEvents := make(<-chan api.CreateInstanceEvent)
	provisioner.EXPECT().CreateInstance(&expectedRequest).Return(createEvents, nil)

	create(machine, unmappableOverlayData)
}

func TestTemplateLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, registry := newRegistry(t, ctrl)

	templates := mock_templates.NewMockTemplates(ctrl)
	templates.EXPECT().Read(provisionerName, templateName).AnyTimes().
		Return(nil, errors.New("Template not found"))

	machine := &machine{registry: registry, templateLoader: templates}
	events, err := create(machine, overlayData)
	require.Nil(t, events)
	require.NotNil(t, err)
}

func TestUnknownProvisiner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, machine := newMachine(t, ctrl, templateData)
	events, err := machine.Create(
		"unknown provisioner",
		provisionerParams,
		templateName,
		overlayData)

	require.Nil(t, events)
	require.NotNil(t, err)
}

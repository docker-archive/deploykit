package libmachete

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provisioners/api"
)

// TaskMap can be used by provisioners to filter and report errors when fetching tasks by name.
type TaskMap struct {
	tasks []api.Task
}

func findTask(tasks []api.Task, name api.TaskName) *api.Task {
	for _, task := range tasks {
		if task.Name == name {
			return &task
		}
	}
	return nil
}

// NewTaskMap creates a TaskMap.
func NewTaskMap(tasks ...api.Task) *TaskMap {
	// Manually implementing map-like behavior here to provide stable return values.

	unique := []api.Task{}
	for _, task := range tasks {
		if findTask(unique, task.Name) != nil {
			panic(fmt.Sprintf("Duplicate task name %s", task))
		} else {
			unique = append(unique, task)
		}
	}

	return &TaskMap{tasks: unique}
}

// Names returns all supported task names.
func (m *TaskMap) Names() []api.TaskName {
	names := []api.TaskName{}
	for _, task := range m.tasks {
		names = append(names, task.Name)
	}
	return names
}

// Filter retrieves tasks by name, returning an error of a requested task does not exist.
func (m *TaskMap) Filter(names []api.TaskName) ([]api.Task, error) {
	filtered := []api.Task{}
	for _, name := range names {
		task := findTask(m.tasks, name)
		if task != nil {
			filtered = append(filtered, *task)
		} else {
			return nil, fmt.Errorf(
				"Task %s is not supported, must be one of %s", name, m.Names())
		}
	}

	return filtered, nil
}

func unimplementedTask(name api.TaskName, desc string) api.Task {
	return api.Task{
		Name:    name,
		Message: desc,
		Do: func(
			prov api.Provisioner,
			keystore api.KeyStore,
			cred api.Credential,
			resource api.Resource,
			req api.MachineRequest,
			events chan<- interface{}) error {
			log.Infoln(fmt.Sprintf("%s: TO BE IMPLEMENTED", name))
			return nil
		},
	}
}

func defaultCreateInstanceHandler(
	prov api.Provisioner,
	keystore api.KeyStore,
	cred api.Credential,
	resource api.Resource,
	req api.MachineRequest,
	events chan<- interface{}) error {

	createInstanceEvents, err := prov.CreateInstance(req)
	if err != nil {
		return err
	}

	for event := range createInstanceEvents {
		events <- event
	}

	return nil
}

func defaultDestroyInstanceHandler(
	prov api.Provisioner,
	keystore api.KeyStore,
	cred api.Credential,
	resource api.Resource,
	req api.MachineRequest,
	events chan<- interface{}) error {

	destroyInstanceEvents, err := prov.DestroyInstance(resource.ID())
	if err != nil {
		return err
	}

	for event := range destroyInstanceEvents {
		events <- event
	}

	return nil
}

// defaultSSHKeyGenHandler is the default task handler that generates a SSH keypair identified by the resource's name.
// If a keypair by the same name already exists, it will emit an error
func defaultSSHKeyGenHandler(prov api.Provisioner, keys api.KeyStore,
	cred api.Credential,
	resource api.Resource,
	req api.MachineRequest,
	events chan<- interface{}) error {

	key := resource.Name()
	if key == "" {
		return fmt.Errorf("Bad resource name")
	}

	if !keys.Exists(key) {
		return keys.NewKeyPair(key)
	}
	return nil
}

// defaultSSHKeyRemoveHandler is the default task handler that will remove the SSH key pair identified by the resource's name.
func defaultSSHKeyRemoveHandler(prov api.Provisioner, keys api.KeyStore,
	cred api.Credential,
	resource api.Resource,
	req api.MachineRequest,
	events chan<- interface{}) error {

	key := resource.Name()
	if key == "" {
		return fmt.Errorf("Bad resource name")
	}

	if keys.Exists(key) {
		return keys.Remove(key)
	}
	return nil
}

var (
	// TaskSSHKeyGen is the task that generates SSH key
	TaskSSHKeyGen = api.Task{
		Name:    "ssh-keygen",
		Message: "Generating ssh key for host",
		Do:      defaultSSHKeyGenHandler,
	}

	// TaskSSHKeyRemove is the task that removes or clean up the SSH key
	TaskSSHKeyRemove = api.Task{
		Name:    "ssh-key-remove",
		Message: "Remove ssh key for host",
		Do:      defaultSSHKeyRemoveHandler,
	}

	// TaskCreateInstance creates a machine instance
	TaskCreateInstance = api.Task{
		Name:    "create-instance",
		Message: "Creates a machine instance",
		Do:      defaultCreateInstanceHandler,
	}

	// TaskDestroyInstance irreversibly destroys a machine instance
	TaskDestroyInstance = api.Task{
		Name:    "destroy-instance",
		Message: "Destroys a machine instance",
		Do:      defaultDestroyInstanceHandler,
	}

	// TaskUserData copies per-instance user data on setup
	TaskUserData = unimplementedTask("user-data", "Copying user data to instance")

	// TaskInstallDockerEngine is the task for installing docker engine.  Requires SSH access.
	TaskInstallDockerEngine = unimplementedTask("install-engine", "Install docker engine")
)

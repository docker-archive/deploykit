package libmachete

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provisioners/api"
	"golang.org/x/net/context"
	"time"
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

// CustomTaskHandler creates a task identical to another task, replacing the handler.
func CustomTaskHandler(task api.Task, handler api.TaskHandler) api.Task {
	task.Do = handler
	return task
}

func unimplementedTask(name api.TaskName, desc string) api.Task {
	return api.Task{
		Name:    name,
		Message: desc,
		Do: func(
			prov api.Provisioner,
			ctx context.Context,
			cred api.Credential,
			req api.MachineRequest,
			events chan<- interface{}) error {

			log.Infoln(fmt.Sprintf("%s: TO BE IMPLEMENTED", name))
			time.Sleep(5 * time.Second)

			events <- fmt.Sprintf(
				"%s: some status here....  need to implement this.", name)
			return nil
		},
	}
}

var (
	// TaskSSHKeyGen is the task that generates SSH key
	TaskSSHKeyGen = unimplementedTask("ssh-keygen", "Generating ssh key for host")

	// TaskCreateInstance creates a machine instance
	TaskCreateInstance = unimplementedTask("create-instance", "Creates a machine instance")

	// TaskUserData copies per-instance user data on setup
	TaskUserData = unimplementedTask("user-data", "Copying user data to instance")

	// TaskInstallDockerEngine is the task for installing docker engine.  Requires SSH access.
	TaskInstallDockerEngine = unimplementedTask("install-engine", "Install docker engine")
)

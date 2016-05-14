package libmachete

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provisioners/api"
	"golang.org/x/net/context"
	"time"
)

var (
	tasks = map[api.TaskType]*api.Task{}
)

// RegisterTask is called by the init of provisioner to register runnable tasks
func RegisterTask(task api.Task) {
	lock.Lock()
	defer lock.Unlock()

	if string(task.Type) == "" {
		panic(fmt.Errorf("Programming error.  No task type name:%v", task))
	}
	if task.Do == nil {
		panic(fmt.Errorf("Programming error.  No task handler :%v", task))
	}

	tasks[task.Type] = &task
}

// GetTask returns the task by type.  The copy of task returned is owned by the caller
// and the caller is free to mutate the task object.
func GetTask(name api.TaskType) (api.Task, bool) {
	t := tasks[name]
	if t != nil {
		copy := *t
		return copy, true
	}
	return api.Task{}, false
}

var (
	// TaskSSHKeyGen is the task that generates SSH key
	TaskSSHKeyGen = api.Task{
		Type:    api.TaskType("ssh-keygen"),
		Message: "Generating ssh key for host",
		Do: func(ctx context.Context, cred api.Credential, req api.MachineRequest, events chan<- interface{}) error {
			log.Infoln("ssh-key-gen: TO BE IMPLEMENTED")
			time.Sleep(5 * time.Second)

			events <- "ssh-key-gen: some status here....  need to implement this."
			return nil
		},
	}

	// TaskCreateInstance creates a machine instance
	TaskCreateInstance = api.Task{
		Type:    api.TaskType("create-instance"),
		Message: "Creates a machine instance",
		Do: func(ctx context.Context, cred api.Credential, req api.MachineRequest, events chan<- interface{}) error {
			log.Infoln("create-instance: TO BE IMPLEMENTED")
			time.Sleep(5 * time.Second)

			events <- "create-instance: some status here....  need to implement this."
			return nil
		},
	}

	// TaskUserData copies per-instance user data on setup
	TaskUserData = api.Task{
		Type:    api.TaskType("user-data"),
		Message: "Copying user data to instance",
		Do: func(ctx context.Context, cred api.Credential, req api.MachineRequest, events chan<- interface{}) error {
			log.Infoln("user-data: TO BE IMPLEMENTED")
			time.Sleep(5 * time.Second)

			events <- "user-data: some status here....  need to implement this."
			return nil
		},
	}

	// TaskInstallEngine is the task for installing docker engine.  Requires SSH access.
	TaskInstallDockerEngine = api.Task{
		Type:    api.TaskType("install-engine"),
		Message: "Install docker engine",
		Do: func(ctx context.Context, cred api.Credential, req api.MachineRequest, events chan<- interface{}) error {
			log.Infoln("install-engine: TO BE IMPLEMENTED")
			time.Sleep(5 * time.Second)

			events <- "installing engine.... implement this."
			return nil
		},
	}
)

// Global tasks
func init() {
	RegisterTask(TaskSSHKeyGen)
	RegisterTask(TaskCreateInstance)
	RegisterTask(TaskUserData)
	RegisterTask(TaskInstallDockerEngine)
}

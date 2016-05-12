package libmachete

import (
	"github.com/docker/libmachete/provisioners/api"
)

var (
	tasks = map[api.TaskName]map[string]*api.Task{}
)

// RegisterTask is called by the init of provisioner to register runnable tasks
func RegisterTask(provisionerName string, task api.Task) {
	lock.Lock()
	defer lock.Unlock()

	if _, has := tasks[task.Name]; !has {
		tasks[task.Name] = map[string]*api.Task{}
	}
	tasks[task.Name][provisionerName] = &task
}

// GetTaskFunc returns the task function by provisioner and task name
func GetTask(provisionerName string, name api.TaskName) *api.Task {
	if m, has := tasks[name]; has {
		if f, has := m[provisionerName]; has {
			return f
		}
	}
	return nil
}

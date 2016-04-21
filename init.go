package libmachete

import (
	"fmt"
	"github.com/docker/libmachete/provisioners/api"
	"sync"
)

var (
	mutex        sync.Mutex
	provisioners = map[string]func(map[string]string) api.Provisioner{}
)

// clear will remove all provisioners, for use in testing.
func clear() {
	mutex.Lock()
	defer mutex.Unlock()

	provisioners = make(map[string]func(map[string]string) api.Provisioner)
}

// RegisterProvisioner makes a Provisioner factory available for use by a canonical short name.
// The factory function will be passed provider-specific details such as credentials.
func RegisterProvisioner(name string, factory func(map[string]string) api.Provisioner) error {
	mutex.Lock()
	defer mutex.Unlock()

	if _, exists := provisioners[name]; exists {
		return fmt.Errorf("A provisioner is already regisered with name '%s'", name)
	}

	provisioners[name] = factory
	return nil
}

// GetProvisioner fetches a provisioenr factory by name.
func GetProvisioner(name string, params map[string]string) api.Provisioner {
	if p, exists := provisioners[name]; exists {
		return p(params)
	}
	return nil
}

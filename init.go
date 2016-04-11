package libmachete

import (
	"sync"
)

var (
	mutex   sync.Mutex
	drivers = map[string]Provisioner{}
)

func Register(driver string, impl Provisioner) {
	mutex.Lock()
	defer mutex.Unlock()
	drivers[driver] = impl
}

func GetProvisioner(driver string) Provisioner {
	if p, exists := drivers[driver]; exists {
		return p
	}
	return nil
}

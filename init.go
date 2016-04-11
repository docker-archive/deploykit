package libmachete

import (
	"sync"
)

var (
	mutex   sync.Mutex
	provisioners = map[string]Provisioner{}
)

// Register makes a Provisioner implementation available for use by a canonical
// short name.
func Register(provisioner string, impl Provisioner) {
	mutex.Lock()
	defer mutex.Unlock()
	provisioners[provisioner] = impl
}

// GetProvisioner fetches a 
func GetProvisioner(provisioner string) Provisioner {
	if p, exists := provisioners[provisioner]; exists {
		return p
	}
	return nil
}

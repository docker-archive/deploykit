package discovery

import (
	"fmt"

	"github.com/docker/infrakit/pkg/plugin"
)

// Plugins provides access to plugin discovery.
type Plugins interface {
	// Find looks up the plugin by name.  The name can be of the form $lookup[/$subtype].  See GetLookupAndType().
	Find(name plugin.Name) (*plugin.Endpoint, error)
	List() (map[string]*plugin.Endpoint, error)
}

const (
	// PluginDirEnvVar is the environment variable that may be used to customize the plugin discovery path.
	PluginDirEnvVar = "INFRAKIT_PLUGINS_DIR"
)

// ErrNotUnixSocketOrListener is the error raised when the file is not a unix socket
type ErrNotUnixSocketOrListener string

func (e ErrNotUnixSocketOrListener) Error() string {
	return fmt.Sprintf("not a unix socket or listener:%s", string(e))
}

// IsErrNotUnixSocketOrListener returns true if the error is due to the file not being a valid unix socket.
func IsErrNotUnixSocketOrListener(e error) bool {
	_, is := e.(ErrNotUnixSocketOrListener)
	return is
}

// ErrNotFound is the error raised when the plugin is not found
type ErrNotFound string

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("plugin not found:%s", string(e))
}

// IsErrNotFound returns true if the error is due to a plugin not found.
func IsErrNotFound(e error) bool {
	_, is := e.(ErrNotFound)
	return is
}

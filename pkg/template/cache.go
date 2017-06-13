package template

import (
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// CacheDirEnvVar is the environment variable used to set the cache of the playbooks
const CacheDirEnvVar = "INFRAKIT_PLAYBOOKS_CACHE"

// default filesystem abstraction
var fs = afero.NewOsFs()

// Setup sets up the necessary environment for running this module -- ie make sure cache directory exists, etc.
func Setup() error {
	dir := Dir()
	if dir == "" {
		return fmt.Errorf("Env not set:%s", CacheDirEnvVar)
	}
	exists, err := afero.Exists(fs, dir)
	if err != nil || !exists {
		log.Debug("Creating directory", "dir", dir)
		err = fs.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}
	// try again
	exists, err = afero.Exists(fs, dir)
	if !exists {
		return fmt.Errorf("Cannot set up directory %s: err=%v", dir, err)
	}
	return err
}

// Dir returns the directory to use for playbooks caching which can be customize via environment variable
func Dir() string {
	if cacheDir := os.Getenv(CacheDirEnvVar); cacheDir != "" {
		return cacheDir
	}

	// if there's INFRAKIT_HOME defined
	home := os.Getenv("INFRAKIT_HOME")
	if home != "" {
		return filepath.Join(home, "playbook-cache")
	}

	home = os.Getenv("HOME")
	if usr, err := user.Current(); err == nil {
		home = usr.HomeDir
	}
	return filepath.Join(home, ".infrakit/playbook-cache")
}

// cache is turned on only when opt.CacheDir is not empty string
func checkCache(p string, opt Options, fetch func() ([]byte, error)) ([]byte, error) {
	if opt.CacheDir == "" {
		return fetch()
	}

	// No need to cache: str:// and file:// since the contents are local.
	if strings.Index("str://", p) == 0 {
		return fetch()
	}
	if strings.Index("file://", p) == 0 {
		return fetch()
	}

	u, err := url.Parse(p)
	if err != nil {
		return nil, err
	}

	path := filepath.Join(opt.CacheDir, u.Path)

	log.Debug("check template cache", "loc", p, "in", opt.CacheDir, "file", path)

	buff, err := afero.ReadFile(fs, path)
	if err != nil {
		buff, err = fetch()
		if err != nil {
			return nil, err
		}
	}

	go func() {
		fs.MkdirAll(filepath.Dir(path), 0755)
		afero.WriteFile(fs, path, buff, 0644)
		log.Debug("template cache written", "loc", p, "in", opt.CacheDir, "file", path)
		return
	}()

	return buff, err
}

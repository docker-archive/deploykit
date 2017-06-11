package template

import (
	"net/url"
	"path/filepath"
	"strings"
)

// GetURL returns a url string of the base and a relative path.
// e.g. http://host/foo/bar/baz, ./boo.tpl gives http://host/foo/bar/boo.tpl
func GetURL(root, rel string) (*url.URL, error) {

	// handle the case when rel is actually a full url
	if strings.Index(rel, "://") > 0 {
		u, err := url.Parse(rel)
		if err != nil {
			return nil, err
		}
		return u, nil
	}

	u, err := url.Parse(root)
	if err != nil {
		return nil, err
	}
	u.Path = filepath.Clean(filepath.Join(filepath.Dir(u.Path), rel))
	return u, nil
}

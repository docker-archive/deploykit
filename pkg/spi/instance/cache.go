package instance

import (
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/types"
)

type cache struct {
	entries map[string]entry
	lock    sync.RWMutex
}

type entry struct {
	value  []Description
	expiry time.Time
}

func (c *cache) put(key string, entry entry) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.entries[key] = entry
}

func (c *cache) delete(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.entries, key)
}

func (c *cache) get(key string) (entry, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	v, has := c.entries[key]
	return v, has
}

// CacheDescribeInstances returns a Plugin that caches the result
// of describe instances.
func CacheDescribeInstances(p Plugin, ttl time.Duration, now func() time.Time) Plugin {
	return &cached{
		Plugin: p,
		ttl:    ttl,
		now:    now,
		cache: &cache{
			entries: map[string]entry{},
		},
	}
}

// Any kind of writes will invalidate the entire cache.  This is for
// simplicity of implementation -- we don't want to remove entries when
// the cache stores entire slice of results.
type cached struct {
	Plugin
	ttl   time.Duration
	now   func() time.Time
	cache *cache
	lock  sync.RWMutex
}

func (c *cached) clear() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache = &cache{
		entries: map[string]entry{},
	}
}

// Provision creates a new instance based on the spec.
func (c *cached) Provision(spec Spec) (*ID, error) {
	// must invalidate the cache
	c.clear()
	return c.Plugin.Provision(spec)
}

// Label labels the instance
func (c *cached) Label(instance ID, labels map[string]string) error {
	// must invalidate the cache
	c.clear()
	return c.Plugin.Label(instance, labels)
}

// Destroy terminates an existing instance.
func (c *cached) Destroy(instance ID, context Context) error {
	// must invalidate the cache
	c.clear()
	return c.Plugin.Destroy(instance, context)
}

// DescribeInstances returns descriptions of all instances matching given tags.
func (c *cached) DescribeInstances(labels map[string]string, properties bool) ([]Description, error) {
	if any, err := types.AnyValue(struct {
		Labels     map[string]string
		Properties bool
	}{
		Labels:     labels,
		Properties: properties,
	}); err == nil {

		// locking here is necessary to prevent multiple threads from all executing the same
		// backend calls.

		c.lock.Lock()
		defer c.lock.Unlock()

		key := types.Fingerprint(any)
		cv, has := c.cache.get(key)
		now := c.now()
		if !has || now.After(cv.expiry) {
			desc, err := c.Plugin.DescribeInstances(labels, properties)
			if err == nil {

				en := entry{value: desc, expiry: now.Add(c.ttl)}
				c.cache.put(key, en)

				return desc, nil
			}
			// just remove cache entry and try later
			c.cache.delete(key)
			return nil, err
		}

		return cv.value, nil
	}

	// when we can't compute a key, just offer a pass-through
	return c.Plugin.DescribeInstances(labels, properties)
}

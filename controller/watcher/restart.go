package watcher

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"golang.org/x/net/context"
	"strings"
	"time"
)

// Restarter retarts a container that matches the given image.
type Restarter struct {
	docker  client.APIClient
	ctx     context.Context
	image   string
	timeout time.Duration
}

// Restart handles restarting containers
func Restart(docker client.APIClient, image string) *Restarter {
	return &Restarter{
		docker:  docker,
		ctx:     context.Background(),
		image:   image,
		timeout: time.Duration(10 * time.Second),
	}
}

// SetTimeout sets the timeout for docker in restarting container.
func (r *Restarter) SetTimeout(t time.Duration) *Restarter {
	r.timeout = t
	return r
}

func imageMatch(a, b string) bool {
	if a == b {
		return true
	}
	// take into account of tag
	aa := strings.Split(a, ":")
	bb := strings.Split(b, ":")
	return aa[0] == bb[0]
}

func (r *Restarter) findContainer() (*types.Container, error) {
	list, err := r.docker.ContainerList(r.ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}

	matches := []*types.Container{}
	for _, c := range list {
		if c.State == "running" && imageMatch(c.Image, r.image) {
			copy := c
			matches = append(matches, &copy)
		}
	}

	// TODO(chungers) -- this needs to be replaced by some means of identifying the
	// exact instance via the plugin manager API.  In that case, we will not be looking
	// for containers but rather specific plugin instances.
	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return matches[0], nil
	default:
		log.Warningln("More than 1 containers ( total=", len(matches), ") matching name=", r.image)
		return matches[0], nil
	}
}

// Run starts the container restart
func (r *Restarter) Run() error {

	r.ctx = context.Background()

	log.Infoln("locating image=", r.image)

	container, err := r.findContainer()
	if err != nil {
		return err
	}

	if container == nil {
		log.Warningln("container not found: image=", r.image)
		return nil
	}

	log.Infoln("restarting container=", container.Image, "id=", container.ID)
	if err := r.docker.ContainerRestart(r.ctx, container.ID, &r.timeout); err != nil {
		return err
	}

	log.Infoln("checking to see the container is running")
	info, err := r.docker.ContainerInspect(r.ctx, container.ID)
	if err != nil {
		return err
	}

	if info.State != nil {
		if info.State.Running {
			log.Infoln("container", r.image, "running. id=", container.ID)
			return nil
		}
	}
	return fmt.Errorf("container %s not running", r.image)
}

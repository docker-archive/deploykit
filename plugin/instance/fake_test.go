package instance

import (
	"context"
	"math/rand"

	"github.com/digitalocean/godo"
	"github.com/pkg/errors"
)

type fakeKeysService struct {
	expectedErr string
	listfunc    func(context.Context, *godo.ListOptions) ([]godo.Key, *godo.Response, error)
}

func (s *fakeKeysService) List(ctx context.Context, opts *godo.ListOptions) ([]godo.Key, *godo.Response, error) {
	if s.expectedErr != "" {
		return nil, nil, errors.New(s.expectedErr)
	}
	if s.listfunc != nil {
		return s.listfunc(ctx, opts)
	}
	return []godo.Key{}, godoResponse(), nil
}

type fakeTagsService struct {
	expectedErr string
}

func (s *fakeTagsService) TagResources(context.Context, string, *godo.TagResourcesRequest) (*godo.Response, error) {
	if s.expectedErr != "" {
		return nil, errors.New(s.expectedErr)
	}
	return nil, nil
}

type fakeDropletsServices struct {
	createfunc  func(context.Context, *godo.DropletCreateRequest) (*godo.Droplet, *godo.Response, error)
	listfunc    func(context.Context, *godo.ListOptions) ([]godo.Droplet, *godo.Response, error)
	expectedErr string
}

func (s *fakeDropletsServices) List(ctx context.Context, opts *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
	if s.expectedErr != "" {
		return nil, nil, errors.New(s.expectedErr)
	}
	if s.listfunc != nil {
		return s.listfunc(ctx, opts)
	}
	return []godo.Droplet{}, godoResponse(), nil
}

func (s *fakeDropletsServices) Get(context.Context, int) (*godo.Droplet, *godo.Response, error) {
	if s.expectedErr != "" {
		return nil, nil, errors.New(s.expectedErr)
	}
	return &godo.Droplet{}, godoResponse(), nil
}

func (s *fakeDropletsServices) Create(ctx context.Context, req *godo.DropletCreateRequest) (*godo.Droplet, *godo.Response, error) {
	if s.expectedErr != "" {
		return nil, nil, errors.New(s.expectedErr)
	}
	if s.createfunc != nil {
		return s.createfunc(ctx, req)
	}
	return &godo.Droplet{}, godoResponse(), nil
}

func (s *fakeDropletsServices) Delete(context.Context, int) (*godo.Response, error) {
	if s.expectedErr != "" {
		return nil, errors.New(s.expectedErr)
	}
	return godoResponse(), nil
}

func godoResponse(ops ...func(*godo.Response)) *godo.Response {
	resp := &godo.Response{
		Links: &godo.Links{},
	}

	for _, op := range ops {
		op(resp)
	}

	return resp
}

func hasNextPage(resp *godo.Response) {
	resp.Links.Pages = &godo.Pages{
		Last: "bar",
	}
}

func godoDroplet(ops ...func(*godo.Droplet)) godo.Droplet {
	droplet := &godo.Droplet{
		ID: rand.Int(),
	}

	for _, op := range ops {
		op(droplet)
	}

	return *droplet
}

func tags(tags ...string) func(*godo.Droplet) {
	return func(droplet *godo.Droplet) {
		droplet.Tags = tags
	}
}

func godoKey(id int, name string, ops ...func(*godo.Key)) godo.Key {
	key := &godo.Key{
		ID:   id,
		Name: name,
	}

	for _, op := range ops {
		op(key)
	}

	return *key
}

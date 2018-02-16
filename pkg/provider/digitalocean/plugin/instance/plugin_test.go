package instance

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/digitalocean/godo"
	itypes "github.com/docker/infrakit/pkg/provider/digitalocean/plugin/instance/types"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLabels(t *testing.T) {
	plugin := &plugin{
		tags: &fakeTagsService{},
	}
	id := instance.ID("foo")
	err := plugin.Label(id, map[string]string{
		"foo":    "bar",
		"banana": "baz",
	})

	require.NoError(t, err)
}

func TestLabelFails(t *testing.T) {
	plugin := &plugin{
		tags: &fakeTagsService{
			expectedErr: "something went wrong",
		},
	}
	id := instance.ID("foo")
	err := plugin.Label(id, map[string]string{
		"foo": "bar",
	})

	require.Error(t, err)
}

func TestBuildCloudInit(t *testing.T) {
	cloudInit, err := buildCloudInit(
		"#!/bin/bash",
		"apt-get update -y; apt-get install -y curl",
		"wget -qO- https://get.docker.com | sh")
	require.NoError(t, err)
	require.Equal(t, `
#cloud-config

runcmd:
- apt-get update -y
- apt-get install -y curl
- wget -qO- https://get.docker.com | sh

`, cloudInit)
}

func TestValidate(t *testing.T) {
	plugin := &plugin{}
	err := plugin.Validate(types.AnyString(`{"Size":"1gb", "Image": "debian-8-x64"}`))

	require.NoError(t, err)
}

func TestValidateFails(t *testing.T) {
	plugin := &plugin{}
	err := plugin.Validate(types.AnyString("-"))

	require.Error(t, err)
}

func TestDestroyFails(t *testing.T) {
	plugin := &plugin{
		droplets: &fakeDropletsServices{
			expectedErr: "something went wrong",
		},
	}
	id := instance.ID("foo")
	err := plugin.Destroy(id, instance.Termination)
	require.Error(t, err)
	_, is := err.(*strconv.NumError)
	require.True(t, is)

	id = instance.ID("12345")
	err = plugin.Destroy(id, instance.Termination)

	require.EqualError(t, err, "something went wrong")
}

func TestDestroy(t *testing.T) {
	// FIXME(vdemeester) make a better test :D
	plugin := &plugin{
		droplets: &fakeDropletsServices{},
	}
	id := instance.ID("12345")
	err := plugin.Destroy(id, instance.Termination)

	require.NoError(t, err)
}

func TestProvisionFailsInvalidProperties(t *testing.T) {
	spec := instance.Spec{
		Properties: types.AnyString(`{
  "NamePrefix": "bar",
  "tags": {
    "foo": "bar",
  }
}`),
	}
	plugin := &plugin{
		droplets: &fakeDropletsServices{},
	}
	_, err := plugin.Provision(spec)
	require.Error(t, err)
}

func TestProvisionFails(t *testing.T) {
	spec := instance.Spec{
		Properties: types.AnyString(`{
  "NamePrefix": "foo",
  "Region" : "asm2",
  "Size": "512mb",
  "Image": { "Slug" : "ubuntu-14-04-x64" },
  "Tags": ["foo"]
}`),
	}
	plugin := &plugin{
		droplets: &fakeDropletsServices{
			expectedErr: "something went wrong",
		},
		keys: &fakeKeysService{},
	}
	_, err := plugin.Provision(spec)
	require.EqualError(t, err, "something went wrong")
}

func TestProvisionFailsWithSshKey(t *testing.T) {
	spec := instance.Spec{
		Properties: types.AnyString(`{
  "NamePrefix": "foo",
  "Region" : "asm2",
  "Size": "512mb",
  "Image": { "Slug" : "ubuntu-14-04-x64" },
  "Tags": ["foo"],
  "SSHKeyNames" : [ "foo" ]
}`),
	}
	plugin := &plugin{
		droplets: &fakeDropletsServices{
			expectedErr: "should not have error out here",
		},
		keys: &fakeKeysService{
			expectedErr: "something went wrong",
		},
	}
	_, err := plugin.Provision(spec)
	require.EqualError(t, err, "something went wrong")
}

func TestProvision(t *testing.T) {
	spec := instance.Spec{
		Properties: types.AnyString(`{
  "NamePrefix": "foo",
  "Region" : "asm2",
  "Size": "512mb",
  "Image": { "Slug" : "ubuntu-14-04-x64" },
  "Tags": ["foo"]
}`),
	}
	versiontag := fmt.Sprintf("%s:%s", itypes.InfrakitDOVersion, itypes.InfrakitDOCurrentVersion)
	plugin := &plugin{
		droplets: &fakeDropletsServices{
			createfunc: func(ctx context.Context, req *godo.DropletCreateRequest) (*godo.Droplet, *godo.Response, error) {
				assert.Contains(t, req.Name, "foo")
				assert.Equal(t, req.Region, "asm2")
				assert.Equal(t, "512mb", req.Size)
				assert.Equal(t, godo.DropletCreateImage{
					Slug: "ubuntu-14-04-x64",
				}, req.Image)
				assert.Condition(t, isInSlice("foo", req.Tags))
				assert.Condition(t, isInSlice(versiontag, req.Tags))
				return &godo.Droplet{
					ID: 12345,
				}, nil, nil
			},
		},
		keys: &fakeKeysService{},
	}
	id, err := plugin.Provision(spec)
	require.NoError(t, err)
	expectedID := instance.ID("12345")
	assert.Equal(t, &expectedID, id)
}

func TestProvisionNonExistingSshkey(t *testing.T) {
	spec := instance.Spec{
		Properties: types.AnyString(`{
  "NamePrefix": "foo",
  "Region" : "asm2",
  "Size": "512mb",
  "Image": { "Slug" : "ubuntu-14-04-x64" },
  "Tags": ["foo"],
  "SSHKeyNames" : [ "foo" ]
}`),
	}
	plugin := &plugin{
		droplets: &fakeDropletsServices{
			createfunc: func(ctx context.Context, req *godo.DropletCreateRequest) (*godo.Droplet, *godo.Response, error) {
				assert.Equal(t, 1, len(req.SSHKeys))
				assert.Equal(t, 0, req.SSHKeys[0].ID)
				return &godo.Droplet{
					ID: 12345,
				}, nil, nil
			},
		},
		keys: &fakeKeysService{
			listfunc: func(context.Context, *godo.ListOptions) ([]godo.Key, *godo.Response, error) {
				return []godo.Key{
					godoKey(54321, "bar"),
				}, godoResponse(), nil
			},
		},
	}
	id, err := plugin.Provision(spec)
	require.NoError(t, err)
	expectedID := instance.ID("12345")
	assert.Equal(t, &expectedID, id)
}

func TestProvisionExistingSshkey(t *testing.T) {
	spec := instance.Spec{
		Properties: types.AnyString(`{
  "NamePrefix": "foo",
  "Region" : "asm2",
  "Size": "512mb",
  "Image": { "Slug" : "ubuntu-14-04-x64" },
  "Tags": ["foo"],
  "SSHKeyNames" : [ "foo" ]
}`),
	}
	plugin := &plugin{
		droplets: &fakeDropletsServices{
			createfunc: func(ctx context.Context, req *godo.DropletCreateRequest) (*godo.Droplet, *godo.Response, error) {
				assert.Equal(t, 1, len(req.SSHKeys))
				assert.Equal(t, 54321, req.SSHKeys[0].ID)
				return &godo.Droplet{
					ID: 12345,
				}, nil, nil
			},
		},
		keys: &fakeKeysService{
			listfunc: func(context.Context, *godo.ListOptions) ([]godo.Key, *godo.Response, error) {
				return []godo.Key{
					godoKey(54321, "foo"),
				}, godoResponse(), nil
			},
		},
	}
	id, err := plugin.Provision(spec)
	require.NoError(t, err)
	expectedID := instance.ID("12345")
	assert.Equal(t, &expectedID, id)
}

func isInSlice(s string, strings []string) assert.Comparison {
	return func() bool {
		isIn := false
		for _, str := range strings {
			if s == str {
				isIn = true
			}
		}
		return isIn
	}
}

func TestDescribeInstancesFails(t *testing.T) {
	plugin := &plugin{
		droplets: &fakeDropletsServices{
			expectedErr: "something went wrong",
		},
	}
	_, err := plugin.DescribeInstances(map[string]string{}, false)
	require.EqualError(t, err, "something went wrong")
}

func TestDescribeInstancesNone(t *testing.T) {
	plugin := &plugin{
		droplets: &fakeDropletsServices{
			listfunc: func(context.Context, *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				return []godo.Droplet{
					godoDroplet(),
					godoDroplet(),
					godoDroplet(),
				}, godoResponse(), nil
			},
		},
	}
	descriptions, err := plugin.DescribeInstances(map[string]string{group.GroupTag: "foo"}, false)

	require.NoError(t, err)
	assert.Len(t, descriptions, 0)
}

func TestDescribeInstances(t *testing.T) {
	plugin := &plugin{
		droplets: &fakeDropletsServices{
			listfunc: func(context.Context, *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				return []godo.Droplet{
					godoDroplet(tags(group.GroupTag + ":foo")),
					godoDroplet(tags(group.GroupTag + ":bar")),
					godoDroplet(tags(group.GroupTag + ":foo")),
				}, godoResponse(), nil
			},
		},
	}
	descriptions, err := plugin.DescribeInstances(map[string]string{group.GroupTag: "foo"}, true)

	require.NoError(t, err)
	assert.Len(t, descriptions, 2)
}

func TestDescribeInstancesHandlesLogicalIDTag(t *testing.T) {
	plugin := &plugin{
		droplets: &fakeDropletsServices{
			listfunc: func(context.Context, *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				return []godo.Droplet{
					godoDroplet(tags(itypes.InfrakitLogicalID + ":foo")),
				}, godoResponse(), nil
			},
		},
	}
	descriptions, err := plugin.DescribeInstances(nil, true)

	require.NoError(t, err)
	assert.Len(t, descriptions, 1)
	if assert.NotNil(t, descriptions[0].LogicalID) {
		assert.Equal(t, instance.LogicalID("foo"), *descriptions[0].LogicalID)
	}
}

func TestDescribeInstancesHandlesMissingLogicalIDTag(t *testing.T) {
	plugin := &plugin{
		droplets: &fakeDropletsServices{
			listfunc: func(context.Context, *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				return []godo.Droplet{
					godoDroplet(tags("foo" + ":bar")),
				}, godoResponse(), nil
			},
		},
	}
	descriptions, err := plugin.DescribeInstances(nil, true)

	require.NoError(t, err)
	assert.Len(t, descriptions, 1)
	assert.Nil(t, descriptions[0].LogicalID)
}

func TestDescribeInstancesHandlesPages(t *testing.T) {
	plugin := &plugin{
		droplets: &fakeDropletsServices{
			listfunc: func(_ context.Context, opts *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				resp := godoResponse(hasNextPage)
				if opts.Page > 0 {
					resp = godoResponse()
				}
				return []godo.Droplet{
					godoDroplet(tags(group.GroupTag + ":foo")),
					godoDroplet(tags(group.GroupTag + ":bar")),
					godoDroplet(tags(group.GroupTag + ":foo")),
				}, resp, nil
			},
		},
	}
	descriptions, err := plugin.DescribeInstances(map[string]string{group.GroupTag: "foo"}, true)

	require.NoError(t, err)
	assert.Len(t, descriptions, 4)
}

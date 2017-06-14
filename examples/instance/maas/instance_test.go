package main

import (
	"github.com/docker/infrakit/pkg/spi/instance"
	maas "github.com/juju/gomaasapi"
	"github.com/stretchr/testify/require"
	"net/url"
	"os"
	"testing"
)

func TestAddansDelTags(t *testing.T) {
	testServer := maas.NewTestServer("1.0")
	defer testServer.Close()
	input := `{"system_id": "test", "hostname":"test", "status": ` + maas.NodeStatusReady + `}`
	testnode := testServer.NewNode(input)
	dir, _ := os.Getwd()
	m := NewMaasPlugin(dir, "", testServer.URL, "1.0")
	//Add Tags Test
	addTags := map[string]string{
		"newlabel1": "newvalue1",
		"newlabel2": "newvalue2",
	}
	requiredTags := addTags
	err := m.(*maasPlugin).addTagsToNode("test", addTags)
	require.NoError(t, err)
	tagObjs, err := testnode.GetMap()["tag_names"].GetArray()
	require.NoError(t, err)
	testNodeTags := map[string]string{}
	for _, tagObj := range tagObjs {
		tag, err := tagObj.GetMAASObject()
		require.NoError(t, err)
		tagname, err := tag.GetField("name")
		require.NoError(t, err)
		tagcomments, err := tag.GetField("comment")
		require.NoError(t, err)
		testNodeTags[tagname] = tagcomments
	}
	require.Equal(t, requiredTags, testNodeTags)

	//Delete Tags Test
	targettags := m.(*maasPlugin).MaasObj.GetSubObject("tags").GetSubObject("newlabel2")
	delTags := []maas.MAASObject{targettags}
	require.NoError(t, err)
	err = m.(*maasPlugin).deleteTagsFromNode("test", delTags)
	require.NoError(t, err)
	delete(requiredTags, "newlabel2")
	tagObjs, err = testnode.GetMap()["tag_names"].GetArray()
	testNodeTags = map[string]string{}
	for _, tagObj := range tagObjs {
		tag, err := tagObj.GetMAASObject()
		require.NoError(t, err)
		tagname, err := tag.GetField("name")
		require.NoError(t, err)
		tagcomments, err := tag.GetField("comment")
		require.NoError(t, err)
		testNodeTags[tagname] = tagcomments
	}
	require.Equal(t, requiredTags, testNodeTags)
}

func TestDelTagsWithoutNode(t *testing.T) {
	testServer := maas.NewTestServer("1.0")
	defer testServer.Close()
	input := `{"system_id": "test", "hostname":"test", "status": ` + maas.NodeStatusReady + `}`
	testnode := testServer.NewNode(input)
	dir, _ := os.Getwd()
	m := NewMaasPlugin(dir, "", testServer.URL, "1.0")
	tagListing := m.(*maasPlugin).MaasObj.GetSubObject("tags")
	tagListing.CallPost("new", url.Values{"name": {"wolabel"}, "comment": {"value"}})

	//Delete Without Tags Test
	targettags, err := m.(*maasPlugin).MaasObj.GetSubObject("tags").GetSubObject("wolabel").Get()
	require.NoError(t, err)
	delTags := []maas.MAASObject{targettags}
	err = m.(*maasPlugin).deleteTagsFromNode("test", delTags)
	require.NoError(t, err)
	tagObjs, err := testnode.GetMap()["tag_names"].GetArray()
	requiredTags := map[string]string{}
	testNodeTags := map[string]string{}
	for _, tagObj := range tagObjs {
		tag, err := tagObj.GetMAASObject()
		require.NoError(t, err)
		tagname, err := tag.GetField("name")
		require.NoError(t, err)
		tagcomments, err := tag.GetField("comment")
		require.NoError(t, err)
		testNodeTags[tagname] = tagcomments
	}
	require.Equal(t, requiredTags, testNodeTags)
}

func TestProvision_and_Destroy(t *testing.T) {
	testServer := maas.NewTestServer("1.0")
	defer testServer.Close()
	dir, _ := os.Getwd()
	maasPlugin := NewMaasPlugin(dir, "", testServer.URL, "1.0")
	instanceSpec := instance.Spec{
		Tags: map[string]string{
			"label1": "value1",
			"label2": "value2",
		},
		Init: "",
	}
	input := `{"system_id": "test", "hostname":"test"}`
	testServer.NewNode(input)

	id, err := maasPlugin.Provision(instanceSpec)
	require.NoError(t, err)

	list, err := maasPlugin.DescribeInstances(map[string]string{"label1": "value1"}, false)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{
		{
			ID: *id,
			Tags: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},
		},
	}, list)
	err = maasPlugin.Label(*id, map[string]string{
		"label1": "value1",
		"label3": "changed",
	})
	require.NoError(t, err)

	list, err = maasPlugin.DescribeInstances(map[string]string{"label1": "value1"}, false)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{
		{
			ID: *id,
			Tags: map[string]string{
				"label1": "value1",
				"label3": "changed",
			},
		},
	}, list)

	list, err = maasPlugin.DescribeInstances(map[string]string{"label3": "changed"}, false)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{
		{
			ID: *id,
			Tags: map[string]string{
				"label1": "value1",
				"label3": "changed",
			},
		},
	}, list)

	err = maasPlugin.Destroy(*id, instance.Termination)
	require.NoError(t, err)
}

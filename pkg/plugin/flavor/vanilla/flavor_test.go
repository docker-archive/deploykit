package vanilla

import (
	"testing"

	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestValidateValid(t *testing.T) {
	plugin := NewPlugin(scope.Nil, DefaultOptions)
	require.NotNil(t, plugin)
	err := plugin.Validate(
		types.AnyString(`{
			"Init": ["l1"],
			"InitScriptTemplateURL": "",
			"Tags": {"tag1": "val1"},
			"Attachments": []
		}`),
		group_types.AllocationMethod{Size: 1})
	require.NoError(t, err)

	err = plugin.Validate(
		types.AnyString(`{
			"InitScriptTemplateURL": "str://l1",
			"Tags": {"tag1": "val1"},
			"Attachments": []
		}`),
		group_types.AllocationMethod{Size: 1})
	require.NoError(t, err)
}

func TestValidateInvalidJSON(t *testing.T) {
	plugin := NewPlugin(scope.Nil, DefaultOptions)
	require.NotNil(t, plugin)
	err := plugin.Validate(
		types.AnyString("not-json"),
		group_types.AllocationMethod{Size: 1})
	require.Error(t, err)
}

func TestValidateInitLinesWithInitScript(t *testing.T) {
	plugin := NewPlugin(scope.Nil, DefaultOptions)
	require.NotNil(t, plugin)
	err := plugin.Validate(
		types.AnyString(`{
			"Init": ["l1"],
			"InitScriptTemplateURL": "str://{{ var \"my-var\" \"value\" }}echo {{ var \"my-var\" }}"
		}`),
		group_types.AllocationMethod{Size: 1})
	require.Error(t, err)
	require.Equal(t,
		"Either \"Init\" or \"InitScriptTemplateURL\" can be specified but not both",
		err.Error())
}

func TestValidateInitScriptRenderError(t *testing.T) {
	plugin := NewPlugin(scope.Nil, DefaultOptions)
	require.NotNil(t, plugin)
	err := plugin.Validate(
		types.AnyString(`{
			"InitScriptTemplateURL": "str://{{ nosuchfunc }}"
		}`),
		group_types.AllocationMethod{Size: 1})
	require.Error(t, err)
}

func TestPrepareEmptyVanillaData(t *testing.T) {
	plugin := NewPlugin(scope.Nil, DefaultOptions)
	require.NotNil(t, plugin)
	spec, err := plugin.Prepare(
		types.AnyString(""),
		instance.Spec{
			Tags:        map[string]string{"t1": "v1"},
			Init:        "l0\nl1",
			Attachments: []instance.Attachment{{ID: "a0"}},
		},
		group_types.AllocationMethod{Size: 1},
		group_types.Index{Group: group.ID("group"), Sequence: 0})
	require.NoError(t, err)
	require.Equal(t, "l0\nl1", spec.Init)
	require.Equal(t, map[string]string{"t1": "v1"}, spec.Tags)
	require.Equal(t, []instance.Attachment{{ID: "a0"}}, spec.Attachments)
}

func TestPrepareWithAttachments(t *testing.T) {
	plugin := NewPlugin(scope.Nil, DefaultOptions)
	require.NotNil(t, plugin)
	spec, err := plugin.Prepare(
		types.AnyString(`{
			"Attachments": [{"ID": "a1"}]
		}`),
		instance.Spec{},
		group_types.AllocationMethod{Size: 1},
		group_types.Index{Group: group.ID("group"), Sequence: 0})
	require.NoError(t, err)
	require.Equal(t, "", spec.Init)
	require.Nil(t, spec.Tags)
	require.Equal(t, []instance.Attachment{{ID: "a1"}}, spec.Attachments)
}

func TestPrepareWithAttachmentsAndInstanceSpecAttachments(t *testing.T) {
	plugin := NewPlugin(scope.Nil, DefaultOptions)
	require.NotNil(t, plugin)
	spec, err := plugin.Prepare(
		types.AnyString(`{
			"Attachments": [{"ID": "a1"}]
		}`),
		instance.Spec{Attachments: []instance.Attachment{{ID: "a0"}}},
		group_types.AllocationMethod{Size: 1},
		group_types.Index{Group: group.ID("group"), Sequence: 0})
	require.NoError(t, err)
	require.Equal(t, "", spec.Init)
	require.Nil(t, spec.Tags)
	require.Equal(t, []instance.Attachment{{ID: "a0"}, {ID: "a1"}}, spec.Attachments)
}

func TestPrepareWithTags(t *testing.T) {
	plugin := NewPlugin(scope.Nil, DefaultOptions)
	require.NotNil(t, plugin)
	spec, err := plugin.Prepare(
		types.AnyString(`{
			"Tags": {"tag1": "val1"}
		}`),
		instance.Spec{},
		group_types.AllocationMethod{Size: 1},
		group_types.Index{Group: group.ID("group"), Sequence: 0})
	require.NoError(t, err)
	require.Equal(t, "", spec.Init)
	require.Equal(t, map[string]string{"tag1": "val1"}, spec.Tags)
}

func TestPrepareWithTagsAndInstanceSpecTags(t *testing.T) {
	plugin := NewPlugin(scope.Nil, DefaultOptions)
	require.NotNil(t, plugin)
	spec, err := plugin.Prepare(
		types.AnyString(`{
			"Tags": {"tag1": "val1"}
		}`),
		instance.Spec{
			Tags: map[string]string{"t1": "v1"},
		},
		group_types.AllocationMethod{Size: 1},
		group_types.Index{Group: group.ID("group"), Sequence: 0})
	require.NoError(t, err)
	require.Equal(t, "", spec.Init)
	require.Equal(t,
		map[string]string{"t1": "v1", "tag1": "val1"},
		spec.Tags)
}

func TestPrepareWithInit(t *testing.T) {
	plugin := NewPlugin(scope.Nil, DefaultOptions)
	require.NotNil(t, plugin)
	spec, err := plugin.Prepare(
		types.AnyString(`{
			"Init": ["line1", "line2"]
		}`),
		instance.Spec{},
		group_types.AllocationMethod{Size: 1},
		group_types.Index{Group: group.ID("group"), Sequence: 0})
	require.NoError(t, err)
	require.Equal(t, "line1\nline2", spec.Init)
	require.Nil(t, spec.Tags)
}

func TestPrepareWithInitAndInstanceSpecInit(t *testing.T) {
	plugin := NewPlugin(scope.Nil, DefaultOptions)
	require.NotNil(t, plugin)
	spec, err := plugin.Prepare(
		types.AnyString(`{
			"Init": ["line2", "line3"]
		}`),
		instance.Spec{
			Init: "l0\nl1",
		},
		group_types.AllocationMethod{Size: 1},
		group_types.Index{Group: group.ID("group"), Sequence: 0})
	require.NoError(t, err)
	require.Equal(t, "l0\nl1\nline2\nline3", spec.Init)
	require.Nil(t, spec.Tags)
}

func TestPrepareWithInitScriptAndInstanceSpecInit(t *testing.T) {
	plugin := NewPlugin(scope.Nil, DefaultOptions)
	require.NotNil(t, plugin)
	spec, err := plugin.Prepare(
		types.AnyString(`{
			"InitScriptTemplateURL": "str://{{ var \"my-var\" \"value\" }}echo {{ var \"my-var\" }}"
		}`),
		instance.Spec{
			Init: "l0\nl1",
		},
		group_types.AllocationMethod{Size: 1},
		group_types.Index{Group: group.ID("group"), Sequence: 0})
	require.NoError(t, err)
	require.Equal(t, "l0\nl1\necho value", spec.Init)
	require.Nil(t, spec.Tags)
}

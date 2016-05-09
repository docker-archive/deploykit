package libmachete

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCodecByContentType(t *testing.T) {
	codec := CodecByContentType("bad")
	require.Equal(t, DefaultContentType, codec)

	codec = CodecByContentType("application/json")
	require.Equal(t, ContentTypeJSON, codec)

	codec = CodecByContentType("application/yaml")
	require.Equal(t, ContentTypeYAML, codec)
}

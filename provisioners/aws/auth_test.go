package aws

import (
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"testing"
)

func TestInvalidEmptyCredential(t *testing.T) {
	cr := NewCredential()
	require.Len(t, cr.(*credential).AccessKeyID, 0)
	require.Len(t, cr.(*credential).SecretAccessKey, 0)

	ctx := BuildContext(context.Background(), "us-west-2")
	require.Error(t, cr.Validate(ctx))
}

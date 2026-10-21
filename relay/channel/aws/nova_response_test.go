package aws

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNovaResponseFirstTextRejectsEmptyContent(t *testing.T) {
	_, err := novaResponseFirstText(novaInvokeResponse{})

	require.Error(t, err)
	require.Contains(t, err.Error(), "empty nova response content")
}

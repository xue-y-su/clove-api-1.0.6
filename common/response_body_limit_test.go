package common

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLimitUpstreamResponseBodyRejectsOversizedPayload(t *testing.T) {
	originalLimit := constant.UpstreamResponseBodyLimitMB
	constant.UpstreamResponseBodyLimitMB = 1
	t.Cleanup(func() {
		constant.UpstreamResponseBodyLimitMB = originalLimit
	})

	body := io.NopCloser(strings.NewReader(strings.Repeat("x", (1<<20)+1)))
	data, err := io.ReadAll(LimitUpstreamResponseBody(body))

	require.Error(t, err)
	var maxBytesError *http.MaxBytesError
	assert.True(t, errors.As(err, &maxBytesError))
	assert.Len(t, data, 1<<20)
}

func TestLimitUpstreamResponseBodyCanBeDisabled(t *testing.T) {
	originalLimit := constant.UpstreamResponseBodyLimitMB
	constant.UpstreamResponseBodyLimitMB = 0
	t.Cleanup(func() {
		constant.UpstreamResponseBodyLimitMB = originalLimit
	})

	body := io.NopCloser(strings.NewReader("complete response"))
	data, err := io.ReadAll(LimitUpstreamResponseBody(body))

	require.NoError(t, err)
	assert.Equal(t, "complete response", string(data))
}

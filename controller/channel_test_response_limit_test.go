package controller

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

func TestReadTestResponseBodyRejectsOversizedNonStreamPayload(t *testing.T) {
	previousLimit := constant.UpstreamResponseBodyLimitMB
	constant.UpstreamResponseBodyLimitMB = 1
	t.Cleanup(func() {
		constant.UpstreamResponseBodyLimitMB = previousLimit
	})

	body := io.NopCloser(strings.NewReader(strings.Repeat("x", (1<<20)+1)))
	data, err := readTestResponseBody(body, false)

	require.Error(t, err)
	var maxBytesError *http.MaxBytesError
	assert.True(t, errors.As(err, &maxBytesError))
	assert.Len(t, data, 1<<20)
}

package main

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestCheckStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		transport  error
		wantError  string
	}{
		{name: "healthy", statusCode: http.StatusOK, body: `{"success":true}`},
		{name: "non-200", statusCode: http.StatusServiceUnavailable, body: `{"success":false}`, wantError: "HTTP 503"},
		{name: "reported failure", statusCode: http.StatusOK, body: `{"success":false}`, wantError: "reported failure"},
		{name: "invalid JSON", statusCode: http.StatusOK, body: `{`, wantError: "decode status response"},
		{name: "transport failure", transport: errors.New("connection failed"), wantError: "request status endpoint"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				assert.Equal(t, statusEndpoint, request.URL.String())
				if test.transport != nil {
					return nil, test.transport
				}
				return &http.Response{
					StatusCode: test.statusCode,
					Body:       io.NopCloser(strings.NewReader(test.body)),
					Header:     make(http.Header),
				}, nil
			})}

			err := checkStatus(client)
			if test.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantError)
		})
	}
}

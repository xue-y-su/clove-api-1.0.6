package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProxyURLStrictNormalizesSupportedURLs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "  ", want: ""},
		{name: "http", raw: " HTTP://proxy.example:8080/ ", want: "http://proxy.example:8080"},
		{name: "https credentials", raw: "https://user:secret@proxy.example:8443", want: "https://user:secret@proxy.example:8443"},
		{name: "socks default port", raw: "socks5://proxy.example", want: "socks5://proxy.example:1080"},
		{name: "socks5h default port", raw: "socks5h://proxy.example/", want: "socks5h://proxy.example:1080"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := ParseProxyURLStrict(test.raw)
			require.NoError(t, err)
			if test.want == "" {
				assert.Nil(t, parsed)
				return
			}
			assert.Equal(t, test.want, parsed.String())
		})
	}
}

func TestParseProxyURLStrictRejectsUnsafeOrAmbiguousURLs(t *testing.T) {
	for _, raw := range []string{
		"ftp://proxy.example",
		"proxy.example:8080",
		"http:///missing-host",
		"http://proxy.example:0",
		"http://proxy.example:65536",
		"http://proxy.example/path",
		"http://proxy.example?token=secret",
		"http://proxy.example#secret",
	} {
		t.Run(raw, func(t *testing.T) {
			parsed, err := ParseProxyURLStrict(raw)
			require.Error(t, err)
			assert.Nil(t, parsed)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
}

func TestParseProxyURLRuntimeStripsLegacySuffix(t *testing.T) {
	parsed, stripped, err := ParseProxyURLRuntime("socks5://user:secret@proxy.example/legacy?token=secret#fragment")

	require.NoError(t, err)
	assert.True(t, stripped)
	assert.Equal(t, "socks5://user:secret@proxy.example:1080", parsed.String())
}

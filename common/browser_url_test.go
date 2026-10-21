package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAllowedBrowserURL(t *testing.T) {
	tests := map[string]bool{
		"https://example.com/console":   true,
		"http://127.0.0.1:3000":         true,
		"https://user:pass@example.com": false,
		"file:///C:/Windows/System32":   false,
		"javascript:alert(1)":           false,
		"//example.com/console":         false,
		"https:///missing-host":         false,
		"not a URL":                     false,
	}

	for rawURL, expected := range tests {
		t.Run(rawURL, func(t *testing.T) {
			assert.Equal(t, expected, isAllowedBrowserURL(rawURL))
		})
	}
}

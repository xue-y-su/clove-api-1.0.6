package ratio_setting

import "testing"

import "github.com/stretchr/testify/assert"

func TestDefaultModelRatioIncludesRC21GPTModels(t *testing.T) {
	tests := map[string]float64{
		"gpt-5.5":       2.5,
		"gpt-5.6-sol":   2.5,
		"gpt-5.6-terra": 1.25,
		"gpt-5.6-luna":  0.5,
	}

	defaults := GetDefaultModelRatioMap()
	for model, want := range tests {
		got, ok := defaults[model]
		assert.True(t, ok, "default ratio for %s must be present", model)
		assert.Equal(t, want, got, "default ratio for %s", model)
	}
}

func TestDefaultModelRatioKeepsSubDollarPerMillionPricesNonZero(t *testing.T) {
	defaults := GetDefaultModelRatioMap()

	assert.InDelta(t, 0.5, defaults["llama-3-sonar-large-32k-chat"], 1e-12)
	assert.InDelta(t, 0.5, defaults["llama-3-sonar-large-32k-online"], 1e-12)
}

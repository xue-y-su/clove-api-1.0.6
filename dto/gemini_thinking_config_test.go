package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiThinkingConfigRejectsBudgetAndLevelTogether(t *testing.T) {
	var config GeminiThinkingConfig

	err := config.UnmarshalJSON([]byte(`{"thinkingBudget":1024,"thinkingLevel":"high"}`))

	require.Error(t, err)
	assert.ErrorContains(t, err, "mutually exclusive")
}

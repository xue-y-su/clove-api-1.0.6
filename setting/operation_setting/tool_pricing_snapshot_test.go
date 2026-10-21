package operation_setting

import (
	"math"
	"sync"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func preserveToolPriceSetting(t *testing.T) {
	t.Helper()
	original := cloneToolPriceSetting(toolPriceSetting)
	t.Cleanup(func() {
		require.NoError(t, replaceToolPriceSetting(original, false))
	})
}

func TestToolPricingSnapshotUsesPublicModelAndLongestPrefix(t *testing.T) {
	preserveToolPriceSetting(t)
	require.NoError(t, ApplyToolPriceSetting(`{
		"enabled": true,
		"prices": {
			"web_search_preview": 10,
			"web_search_preview:gpt-*": 20,
			"web_search_preview:gpt-4.1*": 25
		},
		"multiplier_mode": "group_only",
		"image_pricing_mode": "matrix",
		"image_price_matrix": {},
		"image_model_mode_overrides": {}
	}`))

	snapshot := CurrentToolPricingSnapshot()
	require.NotNil(t, snapshot)
	assert.True(t, snapshot.Enabled())
	assert.True(t, decimal.NewFromInt(25).Equal(snapshot.Price("web_search_preview", "gpt-4.1-mini")))
	assert.True(t, decimal.NewFromInt(10).Equal(snapshot.Price("web_search_preview", "mapped-upstream-name")))
	assert.Equal(t, ToolMultiplierGroupOnly, snapshot.MultiplierMode())
}

func TestToolPriceUpdateRejectsEntireInvalidDocument(t *testing.T) {
	preserveToolPriceSetting(t)
	before := CurrentToolPricingSnapshot()

	err := ApplyToolPriceSetting(`{
		"enabled": true,
		"prices": {"web_search": 10, "bad": -1},
		"multiplier_mode": "group_only",
		"image_pricing_mode": "matrix",
		"image_price_matrix": {},
		"image_model_mode_overrides": {}
	}`)

	require.Error(t, err)
	assert.Same(t, before, CurrentToolPricingSnapshot())
	assert.Equal(t, before.Price("web_search", "gpt-4.1"), CurrentToolPricingSnapshot().Price("web_search", "gpt-4.1"))
}

func TestToolPricingSnapshotSupportsExplicitZeroAndImageModes(t *testing.T) {
	preserveToolPriceSetting(t)
	require.NoError(t, ApplyToolPriceSetting(`{
		"enabled": true,
		"prices": {"web_search": 0, "image_generation": 200},
		"multiplier_mode": "full",
		"image_pricing_mode": "flat",
		"image_price_matrix": {"high": {"1024x1024": 0.5}},
		"image_model_mode_overrides": {"gpt-image-1*": "matrix", "gpt-image-1-mini*": "flat"}
	}`))

	snapshot := CurrentToolPricingSnapshot()
	assert.True(t, snapshot.Price("web_search", "any").IsZero())
	assert.Equal(t, ToolMultiplierFull, snapshot.MultiplierMode())

	price, mode := snapshot.ImagePrice("gpt-image-1", "high", "1024x1024")
	assert.Equal(t, ImagePricingMatrix, mode)
	assert.True(t, decimal.NewFromFloat(0.5).Equal(price))

	price, mode = snapshot.ImagePrice("gpt-image-1-mini-v2", "high", "1024x1024")
	assert.Equal(t, ImagePricingFlat, mode)
	assert.True(t, decimal.NewFromInt(200).Equal(price))
}

func TestToolPricingOptionValidationIsStrict(t *testing.T) {
	valid := map[string]string{
		"enabled":                    "true",
		"prices":                     `{"web_search":0,"custom":1000000}`,
		"multiplier_mode":            "full",
		"image_pricing_mode":         "flat",
		"image_price_matrix":         `{"high":{"1024x1024":100}}`,
		"image_model_mode_overrides": `{"gpt-image-1*":"matrix"}`,
	}
	for key, value := range valid {
		assert.NoError(t, ValidateToolPriceOption(key, value), key)
	}

	invalid := map[string]string{
		"enabled":                    "yes",
		"prices":                     `{"web_search":-1}`,
		"multiplier_mode":            "everything",
		"image_pricing_mode":         "automatic",
		"image_price_matrix":         `{"high":{"1024x1024":-0.1}}`,
		"image_model_mode_overrides": `{"gpt-image-1*":"automatic"}`,
	}
	for key, value := range invalid {
		assert.Error(t, ValidateToolPriceOption(key, value), key)
	}
	assert.Error(t, ValidateToolPriceOption("prices", `{"nan":"NaN"}`))
	assert.Error(t, ValidateToolPriceOption("prices", `{"inf":1e999}`))
	assert.Error(t, ValidateToolPriceOption("unknown", "true"))
	assert.False(t, isValidToolPrice(math.NaN()))
}

func TestToolPricingSnapshotReadersObserveWholeDocuments(t *testing.T) {
	preserveToolPriceSetting(t)
	const first = `{"enabled":true,"prices":{"web_search":11},"multiplier_mode":"group_only","image_pricing_mode":"matrix","image_price_matrix":{},"image_model_mode_overrides":{}}`
	const second = `{"enabled":false,"prices":{"web_search":22},"multiplier_mode":"full","image_pricing_mode":"flat","image_price_matrix":{},"image_model_mode_overrides":{}}`
	require.NoError(t, ApplyToolPriceSetting(first))

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				snapshot := CurrentToolPricingSnapshot()
				price := snapshot.Price("web_search", "model").IntPart()
				if snapshot.Enabled() {
					assert.Equal(t, int64(11), price)
					assert.Equal(t, ToolMultiplierGroupOnly, snapshot.MultiplierMode())
				} else {
					assert.Equal(t, int64(22), price)
					assert.Equal(t, ToolMultiplierFull, snapshot.MultiplierMode())
				}
			}
		}()
	}
	require.NoError(t, ApplyToolPriceSetting(second))
	wg.Wait()
}

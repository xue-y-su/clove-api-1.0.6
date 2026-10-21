package constant

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPath2RelayModeRecognizesPlaygroundImageGeneration(t *testing.T) {
	require.Equal(t, RelayModeImagesGenerations, Path2RelayMode("/pg/images/generations"))
}

func TestPath2RelayModeRecognizesAlphaSearch(t *testing.T) {
	require.Equal(t, RelayModeAlphaSearch, Path2RelayMode("/v1/alpha/search"))
}

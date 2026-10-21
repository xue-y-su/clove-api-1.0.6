package service

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeImageConfigWithLimitRejectsOversizedInput(t *testing.T) {
	_, _, err := decodeImageConfigWithLimit(strings.NewReader(strings.Repeat("x", 65)), 64)

	require.Error(t, err)
	assert.ErrorContains(t, err, "exceeds")
}

func TestDecodeImageConfigWithLimitDecodesBoundedPNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.White)
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, img))

	config, format, err := decodeImageConfigWithLimit(bytes.NewReader(encoded.Bytes()), int64(encoded.Len()))

	require.NoError(t, err)
	assert.Equal(t, "png", format)
	assert.Equal(t, 3, config.Width)
	assert.Equal(t, 2, config.Height)
}

package common

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetOpusDurationIgnoresOutOfRangeGranulePosition(t *testing.T) {
	page := make([]byte, 27)
	copy(page, "OggS")
	binary.LittleEndian.PutUint64(page[6:14], ^uint64(0)-1)

	duration, err := getOpusDuration(bytes.NewReader(page))

	require.NoError(t, err)
	require.Zero(t, duration)
}

func TestDecodeOggGranulePositionRejectsUnsignedOverflow(t *testing.T) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, ^uint64(0)-1)

	_, ok := decodeOggGranulePosition(data)

	require.False(t, ok)
}

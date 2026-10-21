package service

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadISOBMFFBoxSizeRejectsExtendedSizeBeyondBuffer(t *testing.T) {
	box := make([]byte, 16)
	binary.BigEndian.PutUint32(box[:4], 1)
	copy(box[4:8], "meta")
	binary.BigEndian.PutUint64(box[8:16], ^uint64(0))

	_, _, ok := readISOBMFFBoxSize(box, 0)
	require.False(t, ok)
}

func TestReadISOBMFFBoxSizeAcceptsBoundedExtendedSize(t *testing.T) {
	box := make([]byte, 24)
	binary.BigEndian.PutUint32(box[:4], 1)
	copy(box[4:8], "meta")
	binary.BigEndian.PutUint64(box[8:16], 24)

	size, headerLen, ok := readISOBMFFBoxSize(box, 0)
	require.True(t, ok)
	require.Equal(t, 24, size)
	require.Equal(t, 16, headerLen)
}

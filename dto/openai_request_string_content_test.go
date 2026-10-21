package dto

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageStringContentPreservesMediaReferences(t *testing.T) {
	message := Message{
		Role: "user",
		Content: []any{
			map[string]any{"type": "text", "text": "describe this"},
			map[string]any{
				"type": "image_url",
				"image_url": map[string]any{
					"url": "data:image/png;base64,abc123",
				},
			},
			map[string]any{
				"type": "file",
				"file": map[string]any{
					"filename":  "notes.txt",
					"file_data": "data:text/plain;base64,aGVsbG8=",
				},
			},
		},
	}

	content := message.StringContent()

	assert.Contains(t, content, "describe this")
	assert.Contains(t, content, "[image_url: data:image/png;base64,abc123]")
	assert.Contains(t, content, "[file: notes.txt data:text/plain;base64,aGVsbG8=]")
	assert.False(t, strings.Contains(content, "describethis"))
}

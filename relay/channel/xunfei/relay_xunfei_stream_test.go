package xunfei

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeXunfeiReader struct {
	messages [][]byte
	readErr  error
	index    int
}

func (reader *fakeXunfeiReader) ReadMessage() (int, []byte, error) {
	if reader.index < len(reader.messages) {
		message := reader.messages[reader.index]
		reader.index++
		return 1, message, nil
	}
	return 0, nil, reader.readErr
}

func TestConsumeXunfeiResponsesStopsAtTerminalFrame(t *testing.T) {
	reader := &fakeXunfeiReader{
		messages: [][]byte{
			[]byte(`{"header":{"code":0},"payload":{"choices":{"status":0,"text":[{"content":"hello"}]}}}`),
			[]byte(`{"header":{"code":0},"payload":{"choices":{"status":2,"text":[{"content":" world"}]}}}`),
		},
		readErr: errors.New("must not read after terminal frame"),
	}
	var contents []string

	err := consumeXunfeiResponses(reader, func(response XunfeiChatResponse) error {
		contents = append(contents, response.Payload.Choices.Text[0].Content)
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"hello", " world"}, contents)
	assert.Equal(t, 2, reader.index)
}

func TestConsumeXunfeiResponsesPropagatesProtocolAndReadErrors(t *testing.T) {
	tests := []struct {
		name     string
		reader   *fakeXunfeiReader
		contains string
	}{
		{
			name:     "malformed response",
			reader:   &fakeXunfeiReader{messages: [][]byte{[]byte(`{`)}},
			contains: "unexpected end",
		},
		{
			name:     "provider error",
			reader:   &fakeXunfeiReader{messages: [][]byte{[]byte(`{"header":{"code":1001,"message":"invalid request"}}`)}},
			contains: "invalid request",
		},
		{
			name:     "websocket read error",
			reader:   &fakeXunfeiReader{readErr: errors.New("connection lost")},
			contains: "connection lost",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := consumeXunfeiResponses(test.reader, func(XunfeiChatResponse) error { return nil })

			require.Error(t, err)
			assert.Contains(t, err.Error(), test.contains)
		})
	}
}

package ali

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPollAliTaskStopsAfterMaximumAttempts(t *testing.T) {
	attempts := 0

	response, body, err := pollAliTask(context.Background(), 0, 0, 3, func() (*AliResponse, []byte, error) {
		attempts++
		return nil, nil, errors.New("temporary upstream failure")
	})

	require.Error(t, err)
	assert.Nil(t, response)
	assert.Nil(t, body)
	assert.Equal(t, 3, attempts)
}

func TestPollAliTaskHonorsCancellationBeforePolling(t *testing.T) {
	requestContext, cancel := context.WithCancel(context.Background())
	cancel()
	attempts := 0

	response, body, err := pollAliTask(requestContext, 0, 0, 3, func() (*AliResponse, []byte, error) {
		attempts++
		return &AliResponse{}, nil, nil
	})

	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, response)
	assert.Nil(t, body)
	assert.Zero(t, attempts)
}

func TestPollAliTaskReturnsTerminalResponse(t *testing.T) {
	expectedBody := []byte(`{"output":{"task_status":"SUCCEEDED"}}`)

	response, body, err := pollAliTask(context.Background(), 0, 0, 3, func() (*AliResponse, []byte, error) {
		return &AliResponse{Output: AliOutput{TaskStatus: "SUCCEEDED"}}, expectedBody, nil
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "SUCCEEDED", response.Output.TaskStatus)
	assert.Equal(t, expectedBody, body)
}

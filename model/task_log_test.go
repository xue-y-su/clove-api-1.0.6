package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestSafeTaskFailureLogFieldsRedactsPrivateData(t *testing.T) {
	task := Task{
		TaskID:    "task-1",
		UserId:    7,
		ChannelId: 3,
		Platform:  constant.TaskPlatform("video"),
		Status:    TaskStatusFailure,
		PrivateData: TaskPrivateData{
			Key:       "sk-secret",
			ResultURL: "https://example.com/private-video.mp4",
		},
	}

	fields := SafeTaskFailureLogFields(task, "failed upstream")
	payload, err := common.Marshal(fields)
	require.NoError(t, err)

	require.NotContains(t, string(payload), "sk-secret")
	require.NotContains(t, string(payload), "private-video")
	require.Contains(t, string(payload), "task-1")
	require.Contains(t, string(payload), "failed upstream")
}

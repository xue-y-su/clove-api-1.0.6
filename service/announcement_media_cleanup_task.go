package service

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const announcementMediaCleanupInterval = 6 * time.Hour

type announcementMediaCleanupHandler struct{}

func (announcementMediaCleanupHandler) Type() string {
	return model.SystemTaskTypeAnnouncementMediaCleanup
}

func (announcementMediaCleanupHandler) Enabled() bool { return true }

func (announcementMediaCleanupHandler) Interval() time.Duration {
	return announcementMediaCleanupInterval
}

func (announcementMediaCleanupHandler) NewPayload() any { return nil }

func (announcementMediaCleanupHandler) Run(_ context.Context, task *model.SystemTask, runnerID string) {
	removed, err := CleanupCurrentAnnouncementMedia(time.Now())
	status := model.SystemTaskStatusSucceeded
	errorMessage := ""
	if err != nil {
		status = model.SystemTaskStatusFailed
		errorMessage = err.Error()
	}
	if finishErr := model.FinishSystemTask(task.TaskID, runnerID, status, map[string]any{"removed": removed}, errorMessage); finishErr != nil {
		common.SysError("failed to persist announcement media cleanup result: " + finishErr.Error())
	}
}

func init() {
	RegisterSystemTaskHandler(announcementMediaCleanupHandler{})
}

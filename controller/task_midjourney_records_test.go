package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const completeRecordCount = 10_001

type recordPageResponse struct {
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int             `json:"total"`
	Items    json.RawMessage `json:"items"`
}

func setupTaskMidjourneyRecordTest(t *testing.T) *gorm.DB {
	t.Helper()

	db := openTokenControllerTestDB(t)
	db.Logger = db.Logger.LogMode(logger.Silent)
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.Midjourney{}))
	return db
}

func seedCompleteTaskRecordSet(t *testing.T, db *gorm.DB) model.SyncTaskQueryParams {
	t.Helper()

	rows := make([]model.Task, completeRecordCount)
	for i := range rows {
		rows[i] = model.Task{
		TaskID:     "task-filtered",
		Platform:   constant.TaskPlatformSuno,
		UserId:     7,
		ChannelId:  17,
		Action:     "generate",
		Status:     model.TaskStatusSuccess,
		SubmitTime: 150,
		Progress:   "100%",
		}
	}
	require.NoError(t, db.Select(
		"task_id", "platform", "user_id", "channel_id", "action", "status", "submit_time", "progress",
	).CreateInBatches(&rows, 500).Error)

	decoys := []model.Task{
		{TaskID: "other-task", Platform: constant.TaskPlatformSuno, UserId: 7, ChannelId: 17, Action: "generate", Status: model.TaskStatusSuccess, SubmitTime: 150},
		{TaskID: "task-filtered", Platform: constant.TaskPlatformMidjourney, UserId: 7, ChannelId: 17, Action: "generate", Status: model.TaskStatusSuccess, SubmitTime: 150},
		{TaskID: "task-filtered", Platform: constant.TaskPlatformSuno, UserId: 8, ChannelId: 18, Action: "generate", Status: model.TaskStatusSuccess, SubmitTime: 150},
		{TaskID: "task-filtered", Platform: constant.TaskPlatformSuno, UserId: 7, ChannelId: 17, Action: "other", Status: model.TaskStatusSuccess, SubmitTime: 150},
		{TaskID: "task-filtered", Platform: constant.TaskPlatformSuno, UserId: 7, ChannelId: 17, Action: "generate", Status: model.TaskStatusFailure, SubmitTime: 150},
		{TaskID: "task-filtered", Platform: constant.TaskPlatformSuno, UserId: 7, ChannelId: 17, Action: "generate", Status: model.TaskStatusSuccess, SubmitTime: 99},
		{TaskID: "task-filtered", Platform: constant.TaskPlatformSuno, UserId: 7, ChannelId: 17, Action: "generate", Status: model.TaskStatusSuccess, SubmitTime: 201},
	}
	require.NoError(t, db.Create(&decoys).Error)

	return model.SyncTaskQueryParams{
		Platform:       constant.TaskPlatformSuno,
		ChannelID:      "17",
		TaskID:         "task-filtered",
		Action:         "generate",
		Status:         model.TaskStatusSuccess,
		StartTimestamp: 100,
		EndTimestamp:   200,
	}
}

func seedCompleteMidjourneyRecordSet(t *testing.T, db *gorm.DB) model.TaskQueryParams {
	t.Helper()

	rows := make([]model.Midjourney, completeRecordCount)
	for i := range rows {
		rows[i] = model.Midjourney{
		UserId:     7,
		ChannelId:  23,
		MjId:       "mj-filtered",
		SubmitTime: 350,
		Status:     "SUCCESS",
		Progress:   "100%",
		}
	}
	require.NoError(t, db.Select(
		"user_id", "channel_id", "mj_id", "submit_time", "status", "progress",
	).CreateInBatches(&rows, 500).Error)

	decoys := []model.Midjourney{
		{UserId: 8, ChannelId: 24, MjId: "mj-filtered", SubmitTime: 350},
		{UserId: 7, ChannelId: 23, MjId: "other-mj", SubmitTime: 350},
		{UserId: 7, ChannelId: 23, MjId: "mj-filtered", SubmitTime: 299},
		{UserId: 7, ChannelId: 23, MjId: "mj-filtered", SubmitTime: 401},
	}
	require.NoError(t, db.Create(&decoys).Error)

	return model.TaskQueryParams{
		ChannelID:      "23",
		MjID:           "mj-filtered",
		StartTimestamp: "300",
		EndTimestamp:   "400",
	}
}

func TestTaskAndMidjourneyRecordCountsUseCompleteFilteredDataset(t *testing.T) {
	db := setupTaskMidjourneyRecordTest(t)
	taskParams := seedCompleteTaskRecordSet(t, db)
	midjourneyParams := seedCompleteMidjourneyRecordSet(t, db)

	taskTotal, err := model.TaskCountAllTasks(taskParams)
	require.NoError(t, err)
	require.Equal(t, int64(completeRecordCount), taskTotal)
	userTaskTotal, err := model.TaskCountAllUserTask(7, taskParams)
	require.NoError(t, err)
	require.Equal(t, int64(completeRecordCount), userTaskTotal)

	midjourneyTotal, err := model.CountAllTasks(midjourneyParams)
	require.NoError(t, err)
	require.Equal(t, int64(completeRecordCount), midjourneyTotal)
	userMidjourneyTotal, err := model.CountAllUserTask(7, midjourneyParams)
	require.NoError(t, err)
	require.Equal(t, int64(completeRecordCount), userMidjourneyTotal)

	taskCtx, taskRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/task/?p=1&page_size=3&platform=suno&channel_id=17&task_id=task-filtered&action=generate&status=SUCCESS&start_timestamp=100&end_timestamp=200", nil, 1)
	GetAllTask(taskCtx)
	taskResponse := decodeAPIResponse(t, taskRecorder)
	require.True(t, taskResponse.Success, taskResponse.Message)
	var taskPage recordPageResponse
	require.NoError(t, common.Unmarshal(taskResponse.Data, &taskPage))
	require.Equal(t, completeRecordCount, taskPage.Total)
	require.Equal(t, 1, taskPage.Page)
	require.Equal(t, 3, taskPage.PageSize)

	midjourneyCtx, midjourneyRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/mj/?p=1&page_size=3&channel_id=23&mj_id=mj-filtered&start_timestamp=300&end_timestamp=400", nil, 1)
	GetAllMidjourney(midjourneyCtx)
	midjourneyResponse := decodeAPIResponse(t, midjourneyRecorder)
	require.True(t, midjourneyResponse.Success, midjourneyResponse.Message)
	var midjourneyPage recordPageResponse
	require.NoError(t, common.Unmarshal(midjourneyResponse.Data, &midjourneyPage))
	require.Equal(t, completeRecordCount, midjourneyPage.Total)
	require.Equal(t, 1, midjourneyPage.Page)
	require.Equal(t, 3, midjourneyPage.PageSize)
}

func failRecordCountQueries(t *testing.T, db *gorm.DB) error {
	t.Helper()

	failure := errors.New("forced record count query failure")
	const callbackName = "task_midjourney_records_test:fail_count"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil {
			return
		}
		if tx.Statement.Schema.Table != "tasks" && tx.Statement.Schema.Table != "midjourneys" {
			return
		}
		if _, isCount := tx.Statement.Dest.(*int64); isCount {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callbackName) })
	return failure
}

func TestTaskAndMidjourneyRecordAPIsPropagateCountErrors(t *testing.T) {
	testCases := []struct {
		name    string
		path    string
		userID  int
		handler func(*gin.Context)
	}{
		{name: "all tasks", path: "/api/task/?p=1&page_size=10", userID: 1, handler: GetAllTask},
		{name: "user tasks", path: "/api/task/self?p=1&page_size=10", userID: 7, handler: GetUserTask},
		{name: "all midjourney", path: "/api/mj/?p=1&page_size=10", userID: 1, handler: GetAllMidjourney},
		{name: "user midjourney", path: "/api/mj/self?p=1&page_size=10", userID: 7, handler: GetUserMidjourney},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupTaskMidjourneyRecordTest(t)
			failure := failRecordCountQueries(t, db)
			ctx, recorder := newAuthenticatedContext(t, http.MethodGet, testCase.path, nil, testCase.userID)

			testCase.handler(ctx)

			response := decodeAPIResponse(t, recorder)
			require.False(t, response.Success)
			require.Contains(t, response.Message, failure.Error())
			require.Empty(t, response.Data)
		})
	}
}

func failRecordFindQueries(t *testing.T, db *gorm.DB) error {
	t.Helper()

	failure := errors.New("forced record find query failure")
	const callbackName = "task_midjourney_records_test:fail_find"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil {
			return
		}
		if tx.Statement.Schema.Table != "tasks" && tx.Statement.Schema.Table != "midjourneys" {
			return
		}
		if _, isCount := tx.Statement.Dest.(*int64); !isCount {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callbackName) })
	return failure
}

func TestTaskAndMidjourneyRecordAPIsPropagateFindErrors(t *testing.T) {
	testCases := []struct {
		name    string
		path    string
		userID  int
		handler func(*gin.Context)
	}{
		{name: "all tasks", path: "/api/task/?p=1&page_size=10", userID: 1, handler: GetAllTask},
		{name: "user tasks", path: "/api/task/self?p=1&page_size=10", userID: 7, handler: GetUserTask},
		{name: "all midjourney", path: "/api/mj/?p=1&page_size=10", userID: 1, handler: GetAllMidjourney},
		{name: "user midjourney", path: "/api/mj/self?p=1&page_size=10", userID: 7, handler: GetUserMidjourney},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupTaskMidjourneyRecordTest(t)
			failure := failRecordFindQueries(t, db)
			ctx, recorder := newAuthenticatedContext(t, http.MethodGet, testCase.path, nil, testCase.userID)

			testCase.handler(ctx)

			response := decodeAPIResponse(t, recorder)
			require.False(t, response.Success)
			require.Contains(t, response.Message, failure.Error())
			require.Empty(t, response.Data)
		})
	}
}

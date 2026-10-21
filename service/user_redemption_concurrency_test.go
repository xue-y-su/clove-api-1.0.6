package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/user_redemption_setting"
	"github.com/stretchr/testify/require"
)

func TestCreateUserRedemptionEnforcesActiveLimitConcurrentlyAcrossSQLDialects(t *testing.T) {
	testCases := []struct {
		name         string
		environment  string
		databaseType common.DatabaseType
	}{
		{name: "mysql", environment: "TEST_MYSQL_DSN", databaseType: common.DatabaseTypeMySQL},
		{name: "postgres", environment: "TEST_POSTGRES_DSN", databaseType: common.DatabaseTypePostgreSQL},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(testCase.environment))
			if dsn == "" {
				t.Skipf("set %s to run the concurrent active-code limit test", testCase.environment)
			}
			previousDB := model.DB
			previousLogDB := model.LOG_DB
			previousMaster := common.IsMasterNode
			previousMainType := common.MainDatabaseType()
			previousLogType := common.LogDatabaseType()
			t.Setenv("SQL_DSN", dsn)
			t.Setenv("LOG_SQL_DSN", "")
			common.IsMasterNode = true
			require.NoError(t, model.InitDB())
			db := model.DB
			model.LOG_DB = db
			require.Equal(t, testCase.databaseType, common.MainDatabaseType())
			t.Cleanup(func() {
				model.DB = previousDB
				model.LOG_DB = previousLogDB
				common.IsMasterNode = previousMaster
				common.SetDatabaseTypes(previousMainType, previousLogType)
				if sqlDB, dbErr := db.DB(); dbErr == nil {
					require.NoError(t, sqlDB.Close())
				}
			})

			setUserRedemptionTestSettings(t, map[string]string{
				user_redemption_setting.EnabledKey:               "true",
				user_redemption_setting.SingleEnabledKey:         "true",
				user_redemption_setting.RandomEnabledKey:         "true",
				user_redemption_setting.MaxActiveCodesPerUserKey: "1",
			})

			userID := int(time.Now().UnixNano()%100_000_000) + 10_000_000
			unit := redemptionQuota(1)
			username := fmt.Sprintf("redemption-limit-%s-%d", testCase.name, userID)
			require.NoError(t, db.Create(&model.User{
				Id:       userID,
				Username: username,
				AffCode:  fmt.Sprintf("redemption-limit-%d", userID),
				Quota:    unit * 4,
				Status:   common.UserStatusEnabled,
				Role:     common.RoleCommonUser,
			}).Error)
			t.Cleanup(func() {
				codeIDs := db.Model(&model.UserRedemptionCode{}).
					Select("id").Where("creator_user_id = ?", userID)
				_ = db.Where("code_id IN (?)", codeIDs).Delete(&model.UserRedemptionClaim{}).Error
				_ = db.Where("creator_user_id = ?", userID).Delete(&model.UserRedemptionCode{}).Error
				_ = db.Where("user_id = ?", userID).Delete(&model.WalletLedgerEntry{}).Error
				_ = db.Where("actor_id = ? OR target = ?", userID, fmt.Sprintf("user:%d", userID)).Delete(&model.AuditOutboxEvent{}).Error
				_ = db.Unscoped().Where("id = ?", userID).Delete(&model.User{}).Error
			})

			input := CreateUserRedemptionInput{
				CreatorUserID: userID,
				Kind:          model.UserRedemptionKindSingle,
				TotalQuota:    unit,
				MaxClaims:     1,
				ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
			}
			start := make(chan struct{})
			results := make(chan error, 2)
			for range 2 {
				go func() {
					<-start
					_, createErr := CreateUserRedemptionCode(context.Background(), input)
					results <- createErr
				}()
			}
			close(start)

			successes := 0
			limitErrors := 0
			for range 2 {
				createErr := <-results
				switch {
				case createErr == nil:
					successes++
				case errors.Is(createErr, ErrUserRedemptionActiveLimit):
					limitErrors++
				default:
					require.NoError(t, createErr)
				}
			}

			require.Equal(t, 1, successes)
			require.Equal(t, 1, limitErrors)
			var openCodeCount int64
			require.NoError(t, db.Model(&model.UserRedemptionCode{}).
				Where("creator_user_id = ? AND deleted_at = ? AND status IN ?", userID, 0, []string{model.UserRedemptionStatusActive, model.UserRedemptionStatusDisabled}).
				Count(&openCodeCount).Error)
			require.EqualValues(t, 1, openCodeCount)
		})
	}
}

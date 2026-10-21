package model

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestMigrateUserRedemptionReserveOperationUniquenessPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not configured")
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	tests := []struct {
		name              string
		prepareLegacy     bool
		unknownConstraint string
		wantError         bool
	}{
		{name: "fresh"},
		{name: "known_legacy_constraint", prepareLegacy: true},
		{
			name:              "unknown_constraint_is_preserved",
			prepareLegacy:     true,
			unknownConstraint: "user_redemption_codes_reserve_operation_id_key",
			wantError:         true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := db.Begin()
			require.NoError(t, tx.Error)
			t.Cleanup(func() { _ = tx.Rollback().Error })

			schemaName := fmt.Sprintf("user_redemption_migration_%d", time.Now().UnixNano())
			require.NoError(t, tx.Exec(
				"CREATE SCHEMA ?",
				clause.Table{Name: schemaName},
			).Error)
			require.NoError(t, tx.Exec(
				"SET LOCAL search_path TO ?",
				clause.Table{Name: schemaName},
			).Error)

			// migrateDB runs the compatibility migration before the first AutoMigrate.
			require.NoError(t, migrateUserRedemptionReserveOperationUniqueness(tx))
			require.NoError(t, tx.AutoMigrate(&UserRedemptionCode{}))

			original := UserRedemptionCode{
				Code:          "PRESERVED-CODE",
				CreatorUserID: 17,
				IssuedBy:      17,
				RewardType:    UserRedemptionRewardBalance,
				Kind:          UserRedemptionKindSingle,
				TotalQuota:    1000,
				MaxClaims:     1,
				Status:        UserRedemptionStatusActive,
				ExpiresAt:     time.Now().Add(time.Hour).Unix(),
				ReserveOpID:   "reserve:preserved",
			}
			require.NoError(t, tx.Create(&original).Error)

			if test.prepareLegacy {
				require.NoError(t, tx.Exec(
					"ALTER TABLE ? ADD CONSTRAINT ? UNIQUE (?)",
					clause.Table{Name: "user_redemption_codes"},
					clause.Column{Name: legacyUserRedemptionReserveOperationConstraint},
					clause.Column{Name: "reserve_operation_id"},
				).Error)
			}
			if test.unknownConstraint != "" {
				require.NoError(t, tx.Exec(
					"ALTER TABLE ? ADD CONSTRAINT ? UNIQUE (?)",
					clause.Table{Name: "user_redemption_codes"},
					clause.Column{Name: test.unknownConstraint},
					clause.Column{Name: "reserve_operation_id"},
				).Error)
			}

			if test.wantError {
				err := migrateUserRedemptionReserveOperationUniqueness(tx)
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.unknownConstraint)
				assert.True(t, tx.Migrator().HasConstraint(&UserRedemptionCode{}, legacyUserRedemptionReserveOperationConstraint))
				assert.True(t, tx.Migrator().HasConstraint(&UserRedemptionCode{}, test.unknownConstraint))
				return
			}

			for range 2 {
				require.NoError(t, migrateUserRedemptionReserveOperationUniqueness(tx))
				require.NoError(t, tx.AutoMigrate(&UserRedemptionCode{}))
			}

			assert.False(t, tx.Migrator().HasConstraint(&UserRedemptionCode{}, legacyUserRedemptionReserveOperationConstraint))
			assert.True(t, tx.Migrator().HasIndex(&UserRedemptionCode{}, userRedemptionReserveOperationIndex))

			var indexDefinition string
			require.NoError(t, tx.Raw(`
SELECT indexdef
FROM pg_catalog.pg_indexes
WHERE schemaname = current_schema()
  AND tablename = 'user_redemption_codes'
  AND indexname = ?`, userRedemptionReserveOperationIndex).Scan(&indexDefinition).Error)
			assert.Contains(t, strings.ToLower(indexDefinition), "unique index")
			assert.Contains(t, strings.ToLower(indexDefinition), "(reserve_operation_id)")

			var preserved UserRedemptionCode
			require.NoError(t, tx.First(&preserved, original.ID).Error)
			assert.Equal(t, original.Code, preserved.Code)
			assert.Equal(t, original.ReserveOpID, preserved.ReserveOpID)

			duplicate := original
			duplicate.ID = 0
			duplicate.Code = "OTHER-CODE"
			require.Error(t, tx.Transaction(func(duplicateTx *gorm.DB) error {
				return duplicateTx.Create(&duplicate).Error
			}))
		})
	}
}

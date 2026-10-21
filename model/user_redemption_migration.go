package model

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const userRedemptionReserveOperationIndex = "idx_user_redemption_codes_reserve_op_id"
const legacyUserRedemptionReserveOperationConstraint = "idx_user_redemption_codes_reserve_operation_id"

type userRedemptionReserveOperationIndexState struct {
	exists bool
	valid  bool
}

func inspectUserRedemptionReserveOperationConstraints(db *gorm.DB, tableName string) ([]string, error) {
	var constraints []string
	if err := db.Raw(`
SELECT constraint_meta.conname
FROM pg_catalog.pg_constraint AS constraint_meta
WHERE constraint_meta.conrelid = to_regclass(?)
  AND constraint_meta.contype = 'u'
  AND cardinality(constraint_meta.conkey) = 1
  AND EXISTS (
      SELECT 1
      FROM pg_catalog.pg_attribute AS attribute_meta
      WHERE attribute_meta.attrelid = constraint_meta.conrelid
        AND attribute_meta.attnum = constraint_meta.conkey[1]
        AND attribute_meta.attname = ?
  )
ORDER BY constraint_meta.conname`, tableName, "reserve_operation_id").Scan(&constraints).Error; err != nil {
		return nil, fmt.Errorf("inspect user redemption reserve operation unique constraints: %w", err)
	}
	return constraints, nil
}

func validateUserRedemptionReserveOperationConstraints(constraints []string) error {
	unexpected := make([]string, 0)
	for _, name := range constraints {
		if name != legacyUserRedemptionReserveOperationConstraint {
			unexpected = append(unexpected, name)
		}
	}
	if len(unexpected) == 0 {
		return nil
	}
	return fmt.Errorf(
		"user_redemption_codes.reserve_operation_id has unsupported unique constraints %q; only legacy constraint %q can be migrated automatically",
		unexpected,
		legacyUserRedemptionReserveOperationConstraint,
	)
}

func inspectUserRedemptionReserveOperationIndex(db *gorm.DB, tableName string) (userRedemptionReserveOperationIndexState, error) {
	var state struct {
		Exists bool `gorm:"column:index_exists"`
		Valid  bool `gorm:"column:index_valid"`
	}
	if err := db.Raw(`
SELECT count(*) > 0 AS index_exists,
       COALESCE(bool_or(
           index_meta.indisunique
           AND index_meta.indisvalid
           AND index_meta.indisready
           AND NOT index_meta.indisprimary
           AND index_meta.indpred IS NULL
           AND index_meta.indexprs IS NULL
           AND index_meta.indnatts = 1
           AND attribute_meta.attname = ?
       ), false) AS index_valid
FROM pg_catalog.pg_index AS index_meta
JOIN pg_catalog.pg_class AS index_class
  ON index_class.oid = index_meta.indexrelid
LEFT JOIN pg_catalog.pg_attribute AS attribute_meta
  ON attribute_meta.attrelid = index_meta.indrelid
 AND attribute_meta.attnum = index_meta.indkey[0]
WHERE index_meta.indrelid = to_regclass(?)
  AND index_class.relname = ?`, "reserve_operation_id", tableName, userRedemptionReserveOperationIndex).Scan(&state).Error; err != nil {
		return userRedemptionReserveOperationIndexState{}, fmt.Errorf("inspect user redemption reserve operation unique index: %w", err)
	}
	return userRedemptionReserveOperationIndexState{exists: state.Exists, valid: state.Valid}, nil
}

// migrateUserRedemptionReserveOperationUniqueness removes the one known
// redundant PostgreSQL constraint before GORM inspects the column. Unknown
// constraints and incorrectly shaped target indexes are left untouched.
func migrateUserRedemptionReserveOperationUniqueness(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate user redemption reserve operation uniqueness: database is nil")
	}
	if db.Dialector.Name() != "postgres" {
		return nil
	}

	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(&UserRedemptionCode{}); err != nil {
		return fmt.Errorf("parse user redemption code schema: %w", err)
	}
	tableName := statement.Schema.Table
	if !db.Migrator().HasTable(&UserRedemptionCode{}) {
		return nil
	}

	constraints, err := inspectUserRedemptionReserveOperationConstraints(db, tableName)
	if err != nil {
		return err
	}
	if err := validateUserRedemptionReserveOperationConstraints(constraints); err != nil {
		return err
	}
	if len(constraints) == 0 {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"LOCK TABLE ? IN ACCESS EXCLUSIVE MODE",
			clause.Table{Name: tableName},
		).Error; err != nil {
			return fmt.Errorf("lock user redemption codes for reserve operation uniqueness migration: %w", err)
		}

		constraints, err := inspectUserRedemptionReserveOperationConstraints(tx, tableName)
		if err != nil {
			return err
		}
		if err := validateUserRedemptionReserveOperationConstraints(constraints); err != nil {
			return err
		}
		if len(constraints) == 0 {
			return nil
		}

		indexState, err := inspectUserRedemptionReserveOperationIndex(tx, tableName)
		if err != nil {
			return err
		}
		if indexState.exists && !indexState.valid {
			return fmt.Errorf(
				"user redemption reserve operation index %q exists with an unsupported definition",
				userRedemptionReserveOperationIndex,
			)
		}
		if !indexState.exists {
			if err := tx.Migrator().CreateIndex(&UserRedemptionCode{}, userRedemptionReserveOperationIndex); err != nil {
				return fmt.Errorf("create user redemption reserve operation unique index: %w", err)
			}
			indexState, err = inspectUserRedemptionReserveOperationIndex(tx, tableName)
			if err != nil {
				return err
			}
			if !indexState.valid {
				return fmt.Errorf(
					"created user redemption reserve operation index %q does not enforce the expected uniqueness",
					userRedemptionReserveOperationIndex,
				)
			}
		}

		if err := tx.Migrator().DropConstraint(&UserRedemptionCode{}, legacyUserRedemptionReserveOperationConstraint); err != nil {
			return fmt.Errorf("drop legacy user redemption reserve operation constraint: %w", err)
		}
		return nil
	})
}

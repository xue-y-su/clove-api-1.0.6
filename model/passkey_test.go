package model

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGetPasskeyByUserIDDoesNotLogExpectedUnboundUser(t *testing.T) {
	var output bytes.Buffer
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.New(log.New(&output, "", 0), logger.Config{
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: false,
			Colorful:                  false,
		}),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PasskeyCredential{}))

	previousDB := DB
	DB = db
	t.Cleanup(func() {
		DB = previousDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	output.Reset()

	credential, err := GetPasskeyByUserID(42)
	require.Nil(t, credential)
	require.ErrorIs(t, err, ErrPasskeyNotFound)
	require.NotContains(t, output.String(), "record not found")
}

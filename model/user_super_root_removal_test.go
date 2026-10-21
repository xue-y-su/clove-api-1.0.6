package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUserJSONDoesNotExposeSuperRoot(t *testing.T) {
	data, err := common.Marshal(User{Id: 1, Username: "root", Role: common.RoleRootUser})
	require.NoError(t, err)
	assert.NotContains(t, string(data), "super_root")
}

func TestUserSchemaDoesNotCreateSuperRootColumn(t *testing.T) {
	dsn := "file:super-root-removal?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}))

	columns, err := db.Migrator().ColumnTypes(&User{})
	require.NoError(t, err)
	for _, column := range columns {
		assert.NotEqual(t, "super_root", strings.ToLower(column.Name()))
	}
}

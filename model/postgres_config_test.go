package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPostgresGormConfigDisablesPreparedStatements(t *testing.T) {
	assert.False(t, newPostgresGormConfig().PrepareStmt)
}

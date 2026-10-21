package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateOptionBeforePersistenceRejectsInvalidMaxTokenAutoGroups(t *testing.T) {
	for _, value := range []string{"", "0", "-1", "1.5", "invalid"} {
		t.Run(value, func(t *testing.T) {
			assert.Error(t, validateOptionBeforePersistence("MaxTokenAutoGroups", value))
		})
	}
	require.NoError(t, validateOptionBeforePersistence("MaxTokenAutoGroups", "999999"))
}

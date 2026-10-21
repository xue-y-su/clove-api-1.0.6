package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuditContentENRendersCloveSecurityAndAnnouncementActions(t *testing.T) {
	tests := map[string]string{
		"announcement.acknowledge":     "Acknowledged the latest announcement",
		"announcement.media_upload":    "Uploaded an announcement image",
		"announcement.settings_update": "Updated announcement settings",
		"security.settings_update":     "Updated security settings",
		"user.profile_update":          "Updated personal profile",
	}

	for action, expected := range tests {
		t.Run(action, func(t *testing.T) {
			assert.Equal(t, expected, auditContentEN(action, nil))
		})
	}
}

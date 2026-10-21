package console_setting

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestValidateAnnouncementsAllowsHTTPSImage(t *testing.T) {
	payload, err := common.Marshal([]map[string]any{{
		"id": 1, "content": "Maintenance", "publishDate": "2026-07-20T00:00:00Z",
		"type": "warning", "imageUrl": "https://img.cdn1.vip/i/notice.png",
	}})
	require.NoError(t, err)
	require.NoError(t, ValidateConsoleSettings(string(payload), "Announcements"))
}

func TestValidateAnnouncementsAllowsManagedImage(t *testing.T) {
	payload, err := common.Marshal([]map[string]any{{
		"id": 1, "content": "Maintenance", "publishDate": "2026-07-20T00:00:00Z",
		"type": "warning", "imageUrl": "/api/announcement-media/" + strings.Repeat("a", 64) + ".png",
	}})
	require.NoError(t, err)
	require.NoError(t, ValidateConsoleSettings(string(payload), "Announcements"))
}

func TestValidateAnnouncementsRejectsInvalidManagedImage(t *testing.T) {
	for _, imageURL := range []string{
		"/api/announcement-media/notice.png",
		"/api/announcement-media/" + strings.Repeat("A", 64) + ".png",
		"/api/announcement-media/" + strings.Repeat("a", 64) + ".svg",
		"/api/announcement-media/../" + strings.Repeat("a", 64) + ".png",
	} {
		payload, err := common.Marshal([]map[string]any{{
			"id": 1, "content": "Maintenance", "publishDate": "2026-07-20T00:00:00Z",
			"type": "warning", "imageUrl": imageURL,
		}})
		require.NoError(t, err)
		require.Error(t, ValidateConsoleSettings(string(payload), "Announcements"), imageURL)
	}
}

func TestValidateAnnouncementsRejectsUnsafeImageURL(t *testing.T) {
	for _, imageURL := range []string{
		"http://img.example.com/notice.png",
		"https://localhost/notice.png",
		"https://127.0.0.1/notice.png",
	} {
		payload, err := common.Marshal([]map[string]any{{
			"id": 1, "content": "Maintenance", "publishDate": "2026-07-20T00:00:00Z",
			"type": "warning", "imageUrl": imageURL,
		}})
		require.NoError(t, err)
		require.Error(t, ValidateConsoleSettings(string(payload), "Announcements"), imageURL)
	}
}

func TestValidateAnnouncementsRejectsInvalidOptionalFieldTypes(t *testing.T) {
	for field, value := range map[string]any{
		"type":  7,
		"extra": 7,
	} {
		payload, err := common.Marshal([]map[string]any{{
			"id": 1, "content": "Maintenance", "publishDate": "2026-07-20T00:00:00Z",
			field: value,
		}})
		require.NoError(t, err)
		require.Error(t, ValidateConsoleSettings(string(payload), "Announcements"), field)
	}
}

func TestValidateAnnouncementsAllowsTwentyThousandChineseCharacters(t *testing.T) {
	payload, err := common.Marshal([]map[string]any{{
		"id": 1, "content": strings.Repeat("你", 20_000), "publishDate": "2026-07-20T00:00:00Z",
		"type": "warning", "popup": true,
	}})
	require.NoError(t, err)
	require.NoError(t, ValidateConsoleSettings(string(payload), "Announcements"))
}

func TestValidateAnnouncementsIgnoresLegacyContentFormat(t *testing.T) {
	payload, err := common.Marshal([]map[string]any{{
		"id": 1, "content": "Maintenance", "publishDate": "2026-07-20T00:00:00Z",
		"content_format": "rich-text",
	}})
	require.NoError(t, err)
	require.NoError(t, ValidateConsoleSettings(string(payload), "Announcements"))
}

func TestValidateAnnouncementsRejectsContentOver64KiB(t *testing.T) {
	payload, err := common.Marshal([]map[string]any{{
		"id": 1, "content": strings.Repeat("a", 64*1024+1), "publishDate": "2026-07-20T00:00:00Z",
	}})
	require.NoError(t, err)
	require.ErrorContains(t, ValidateConsoleSettings(string(payload), "Announcements"), "64 KiB")
}

func TestValidateAnnouncementsAllowsAggregatePayloadAboveLegacyLimit(t *testing.T) {
	announcements := make([]map[string]any, 5)
	for index := range announcements {
		announcements[index] = map[string]any{
			"id": index + 1, "content": strings.Repeat("a", 60*1024),
			"publishDate": "2026-07-20T00:00:00Z",
		}
	}
	payload, err := common.Marshal(announcements)
	require.NoError(t, err)
	require.NoError(t, ValidateConsoleSettings(string(payload), "Announcements"))
}

func TestNormalizeAnnouncementsJSONSanitizesHTMLAndDefaultsLegacyPublishing(t *testing.T) {
	raw := `[{
		"id":1,
		"title":"  <b>Release</b>  ",
		"content":"<p>Hello <script>alert(1)</script><strong>world</strong><a href=\"javascript:alert(2)\" onclick=\"alert(3)\">bad</a><a href=\"https://example.com/docs\" target=\"_blank\">good</a></p>",
		"extra":"<style>body{display:none}</style><em>Read this</em>",
		"publishDate":"2026-07-20T00:00:00Z",
		"content_format":"html"
	},{
		"id":2,
		"content":"Hidden",
		"publishDate":"2026-07-20T00:00:00Z",
		"published":false
	}]`

	normalized, err := NormalizeAnnouncementsJSON(raw)
	require.NoError(t, err)
	var announcements []map[string]any
	require.NoError(t, common.Unmarshal([]byte(normalized), &announcements))
	require.Len(t, announcements, 2)

	first := announcements[0]
	require.Equal(t, "Release", first["title"])
	require.Equal(t, true, first["published"])
	require.Equal(t, "default", first["type"])
	require.NotContains(t, first, "content_format")
	content := first["content"].(string)
	require.Contains(t, content, "<strong>world</strong>")
	require.Contains(t, content, `href="https://example.com/docs"`)
	require.Contains(t, content, `rel="noopener noreferrer"`)
	require.NotContains(t, content, "script")
	require.NotContains(t, content, "javascript:")
	require.NotContains(t, content, "onclick")
	require.Equal(t, "<em>Read this</em>", first["extra"])
	require.Equal(t, false, announcements[1]["published"])
}

func TestGetAnnouncementsAtReturnsOnlyPublishedReleasedItems(t *testing.T) {
	previous := GetConsoleSetting().Announcements
	t.Cleanup(func() { GetConsoleSetting().Announcements = previous })
	GetConsoleSetting().Announcements = `[
		{"id":1,"title":"Legacy","content":"Legacy","publishDate":"2026-07-20T00:00:00Z"},
		{"id":2,"title":"Hidden","content":"Hidden","publishDate":"2026-07-21T00:00:00Z","published":false},
		{"id":3,"title":"Future","content":"Future","publishDate":"2026-07-23T00:00:00Z","published":true},
		{"id":4,"title":"Current","content":"Current","publishDate":"2026-07-22T00:00:00Z","published":true}
	]`

	announcements := getAnnouncementsAt(time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC))
	require.Len(t, announcements, 2)
	require.Equal(t, float64(4), announcements[0]["id"])
	require.Equal(t, float64(1), announcements[1]["id"])
}

func TestValidateAnnouncementsChecksTitleAndPublishedTypes(t *testing.T) {
	for field, value := range map[string]any{
		"title":     7,
		"published": "yes",
	} {
		payload, err := common.Marshal([]map[string]any{{
			"id": 1, "content": "Maintenance", "publishDate": "2026-07-20T00:00:00Z",
			field: value,
		}})
		require.NoError(t, err)
		require.Error(t, ValidateConsoleSettings(string(payload), "Announcements"), field)
	}
}

func TestValidateAnnouncementsCountsTitleLengthByUnicodeCharacters(t *testing.T) {
	validPayload, err := common.Marshal([]map[string]any{{
		"id": 1, "title": strings.Repeat("公", 200), "content": "Maintenance",
		"publishDate": "2026-07-20T00:00:00Z",
	}})
	require.NoError(t, err)
	require.NoError(t, ValidateConsoleSettings(string(validPayload), "Announcements"))

	invalidPayload, err := common.Marshal([]map[string]any{{
		"id": 1, "title": strings.Repeat("公", 201), "content": "Maintenance",
		"publishDate": "2026-07-20T00:00:00Z",
	}})
	require.NoError(t, err)
	require.Error(t, ValidateConsoleSettings(string(invalidPayload), "Announcements"))
}

package controller

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	appi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type tokenAPIResponse struct {
	Success   bool            `json:"success"`
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}

type tokenPageResponse struct {
	Items []tokenResponseItem `json:"items"`
}

type tokenResponseItem struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Key          string `json:"key"`
	Status       int    `json:"status"`
	StatusReason string `json:"status_reason"`
}

type tokenKeyResponse struct {
	Key string `json:"key"`
}

type sqliteColumnInfo struct {
	Name string `gorm:"column:name"`
	Type string `gorm:"column:type"`
}

type legacyToken struct {
	Id                 int    `gorm:"primaryKey"`
	UserId             int    `gorm:"index"`
	Key                string `gorm:"column:key;type:char(48);uniqueIndex"`
	Status             int    `gorm:"default:1"`
	Name               string `gorm:"index"`
	CreatedTime        int64  `gorm:"bigint"`
	AccessedTime       int64  `gorm:"bigint"`
	ExpiredTime        int64  `gorm:"bigint;default:-1"`
	RemainQuota        int    `gorm:"default:0"`
	UnlimitedQuota     bool
	ModelLimitsEnabled bool
	ModelLimits        string  `gorm:"type:text"`
	AllowIps           *string `gorm:"default:''"`
	UsedQuota          int     `gorm:"default:0"`
	Group              string  `gorm:"column:group;default:''"`
	CrossGroupRetry    bool
	DeletedAt          gorm.DeletedAt `gorm:"index"`
}

func (legacyToken) TableName() string {
	return "tokens"
}

func openTokenControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func migrateTokenControllerTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.AutoMigrate(
		&model.Token{}, &model.RevokedTokenFingerprint{}, &model.User{}, &model.Log{}, &model.AuditOutboxEvent{},
		&model.IPSecurityEvent{}, &model.IPAuditRollup{}, &model.IPSecurityProjectionState{},
	); err != nil {
		t.Fatalf("failed to migrate token table: %v", err)
	}
	if err := model.EnsureTokenSecretIndexes(db); err != nil {
		t.Fatalf("failed to migrate token secret indexes: %v", err)
	}
}

func setupTokenControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	require.NoError(t, appi18n.Init())

	db := openTokenControllerTestDB(t)
	migrateTokenControllerTestDB(t, db)
	return db
}

func configureTokenSecretManagerForControllerTest(t *testing.T) {
	t.Helper()
	require.NoError(t, service.ConfigureTokenSecretManager(common.TokenSecretConfig{
		Phase:   common.TokenSecretPhaseDualWrite,
		HMACKey: []byte(strings.Repeat("h", 32)),
		EncryptionKeys: []common.TokenEncryptionKey{
			{Version: 1, Key: []byte(strings.Repeat("a", 32))},
		},
	}))
	t.Cleanup(func() {
		require.NoError(t, service.ConfigureTokenSecretManager(common.TokenSecretConfig{Phase: common.TokenSecretPhaseSchema}))
	})
}

func openTokenControllerExternalDB(t *testing.T, dialect string, dsn string) (*gorm.DB, *bool) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false

	var (
		db     *gorm.DB
		dbType common.DatabaseType
		err    error
	)
	switch dialect {
	case "mysql":
		dbType = common.DatabaseTypeMySQL
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case "postgres":
		dbType = common.DatabaseTypePostgreSQL
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	default:
		t.Fatalf("unsupported dialect %q", dialect)
	}
	common.SetDatabaseTypes(dbType, dbType)
	if err != nil {
		t.Fatalf("failed to open %s db: %v", dialect, err)
	}

	model.DB = db
	model.LOG_DB = db

	if db.Migrator().HasTable("tokens") {
		t.Skipf("refusing to run %s migration compatibility test against external database because tokens table already exists", dialect)
	}

	managedTokensTable := new(bool)

	t.Cleanup(func() {
		if *managedTokensTable && db.Migrator().HasTable("tokens") {
			_ = db.Migrator().DropTable("tokens")
		}
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db, managedTokensTable
}

func seedToken(t *testing.T, db *gorm.DB, userID int, name string, rawKey string) *model.Token {
	t.Helper()

	token := &model.Token{
		UserId:         userID,
		Name:           name,
		Key:            rawKey,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	return token
}

func seedTokenOwner(t *testing.T, db *gorm.DB, id int, username string, email string) *model.User {
	t.Helper()

	user := &model.User{
		Id:          id,
		Username:    username,
		Password:    "hashed-password",
		DisplayName: username,
		Email:       email,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     fmt.Sprintf("aff-%d", id),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create token owner: %v", err)
	}
	return user
}

func newAuthenticatedContext(t *testing.T, method string, target string, body any, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var requestBody *bytes.Reader
	if body != nil {
		payload, err := common.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		requestBody = bytes.NewReader(payload)
	} else {
		requestBody = bytes.NewReader(nil)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, requestBody)
	if body != nil {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	ctx.Set("id", userID)
	return ctx, recorder
}

func decodeAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) tokenAPIResponse {
	t.Helper()

	var response tokenAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode api response: %v", err)
	}
	return response
}

func getSQLiteColumnType(t *testing.T, db *gorm.DB, tableName string, columnName string) string {
	t.Helper()

	var columns []sqliteColumnInfo
	if err := db.Raw("PRAGMA table_info(" + tableName + ")").Scan(&columns).Error; err != nil {
		t.Fatalf("failed to inspect %s schema: %v", tableName, err)
	}

	for _, column := range columns {
		if column.Name == columnName {
			return strings.ToLower(column.Type)
		}
	}

	t.Fatalf("column %s not found in %s schema", columnName, tableName)
	return ""
}

func getTokenKeyColumnType(t *testing.T, db *gorm.DB, dialect string) string {
	t.Helper()

	switch dialect {
	case "sqlite":
		return getSQLiteColumnType(t, db, "tokens", "key")
	case "mysql":
		var columnType string
		if err := db.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
			WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			"tokens", "key").Scan(&columnType).Error; err != nil {
			t.Fatalf("failed to inspect mysql token key column: %v", err)
		}
		return strings.ToLower(columnType)
	case "postgres":
		var dataType string
		var maxLength sql.NullInt64
		if err := db.Raw(`SELECT data_type, character_maximum_length
			FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			"tokens", "key").Row().Scan(&dataType, &maxLength); err != nil {
			t.Fatalf("failed to inspect postgres token key column: %v", err)
		}
		switch strings.ToLower(dataType) {
		case "character varying":
			return fmt.Sprintf("varchar(%d)", maxLength.Int64)
		case "character":
			return fmt.Sprintf("char(%d)", maxLength.Int64)
		default:
			if maxLength.Valid {
				return fmt.Sprintf("%s(%d)", strings.ToLower(dataType), maxLength.Int64)
			}
			return strings.ToLower(dataType)
		}
	default:
		t.Fatalf("unsupported dialect %q", dialect)
		return ""
	}
}

func runTokenMigrationCompatibilityTest(t *testing.T, db *gorm.DB, dialect string, managedTokensTable *bool) {
	t.Helper()

	legacyKey := strings.Repeat("a", 48)
	longKey := strings.Repeat("b", 64)

	if err := db.AutoMigrate(&legacyToken{}); err != nil {
		t.Fatalf("failed to create legacy token schema: %v", err)
	}
	if managedTokensTable != nil {
		*managedTokensTable = true
	}
	if err := db.Create(&legacyToken{
		UserId:             7,
		Key:                legacyKey,
		Status:             common.TokenStatusEnabled,
		Name:               "legacy-token",
		CreatedTime:        1,
		AccessedTime:       1,
		ExpiredTime:        -1,
		RemainQuota:        100,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
		ModelLimits:        "",
		AllowIps:           common.GetPointer(""),
		UsedQuota:          0,
		Group:              "default",
		CrossGroupRetry:    false,
	}).Error; err != nil {
		t.Fatalf("failed to seed legacy token row: %v", err)
	}

	if got := getTokenKeyColumnType(t, db, dialect); got != "char(48)" {
		t.Fatalf("expected legacy key column type char(48), got %q", got)
	}

	migrateTokenControllerTestDB(t, db)

	if got := getTokenKeyColumnType(t, db, dialect); got != "varchar(128)" {
		t.Fatalf("expected migrated key column type varchar(128), got %q", got)
	}

	var migratedToken model.Token
	if err := db.First(&migratedToken, "name = ?", "legacy-token").Error; err != nil {
		t.Fatalf("failed to load migrated token row: %v", err)
	}
	if migratedToken.Key != legacyKey {
		t.Fatalf("expected migrated token key %q, got %q", legacyKey, migratedToken.Key)
	}
	if migratedToken.Name != "legacy-token" {
		t.Fatalf("expected migrated token name to be preserved, got %q", migratedToken.Name)
	}

	inserted := model.Token{
		UserId:             8,
		Name:               "long-token",
		Key:                longKey,
		Status:             common.TokenStatusEnabled,
		CreatedTime:        1,
		AccessedTime:       1,
		ExpiredTime:        -1,
		RemainQuota:        200,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
		ModelLimits:        "",
		AllowIps:           common.GetPointer(""),
		UsedQuota:          0,
		Group:              "default",
		CrossGroupRetry:    false,
	}
	if err := db.Create(&inserted).Error; err != nil {
		t.Fatalf("failed to insert long token after migration: %v", err)
	}

	var fetched model.Token
	if err := db.First(&fetched, "id = ?", inserted.Id).Error; err != nil {
		t.Fatalf("failed to fetch long token after migration: %v", err)
	}
	if fetched.Key != longKey {
		t.Fatalf("expected long token key %q, got %q", longKey, fetched.Key)
	}
}

func TestTokenAutoMigrateUsesVarchar128KeyColumn(t *testing.T) {
	db := setupTokenControllerTestDB(t)

	if got := getTokenKeyColumnType(t, db, "sqlite"); got != "varchar(128)" {
		t.Fatalf("expected key column type varchar(128), got %q", got)
	}
}

func TestTokenMigrationFromChar48ToVarchar128(t *testing.T) {
	db := openTokenControllerTestDB(t)
	runTokenMigrationCompatibilityTest(t, db, "sqlite", nil)
}

func TestTokenMigrationFromChar48ToVarchar128MySQL(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run mysql migration compatibility test")
	}

	db, managedTokensTable := openTokenControllerExternalDB(t, "mysql", dsn)
	runTokenMigrationCompatibilityTest(t, db, "mysql", managedTokensTable)
}

func TestTokenMigrationFromChar48ToVarchar128Postgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TEST_POSTGRES_DSN to run postgres migration compatibility test")
	}

	db, managedTokensTable := openTokenControllerExternalDB(t, "postgres", dsn)
	runTokenMigrationCompatibilityTest(t, db, "postgres", managedTokensTable)
}

func TestGetAllTokensMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "list-token", "abcd1234efgh5678")
	seedToken(t, db, 2, "other-user-token", "zzzz1234yyyy5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/?p=1&size=10", nil, 1)
	GetAllTokens(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page tokenPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode token page response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one token, got %d", len(page.Items))
	}
	if page.Items[0].Key != token.GetMaskedKey() {
		t.Fatalf("expected masked key %q, got %q", token.GetMaskedKey(), page.Items[0].Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("list response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestSearchTokensMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "searchable-token", "ijkl1234mnop5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/search?keyword=searchable-token&p=1&size=10", nil, 1)
	SearchTokens(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page tokenPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode search response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one search result, got %d", len(page.Items))
	}
	if page.Items[0].Key != token.GetMaskedKey() {
		t.Fatalf("expected masked search key %q, got %q", token.GetMaskedKey(), page.Items[0].Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("search response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestGetTokenMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "detail-token", "qrst1234uvwx5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/"+strconv.Itoa(token.Id), nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var detail tokenResponseItem
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode token detail response: %v", err)
	}
	if detail.Key != token.GetMaskedKey() {
		t.Fatalf("expected masked detail key %q, got %q", token.GetMaskedKey(), detail.Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("detail response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestUpdateTokenMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "editable-token", "yzab1234cdef5678")

	body := map[string]any{
		"id":                   token.Id,
		"name":                 "updated-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var detail tokenResponseItem
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode token update response: %v", err)
	}
	if detail.Key != token.GetMaskedKey() {
		t.Fatalf("expected masked update key %q, got %q", token.GetMaskedKey(), detail.Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("update response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestUpdateTokenGroupOnlyChangesGroupAndWritesAudit(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenOwner(t, db, 1, "owner", "owner@example.com")
	previousGroups := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousGroups))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))

	token := seedToken(t, db, 1, "old-name", "group-only-key")
	token.RemainQuota = 900
	token.UnlimitedQuota = false
	token.ExpiredTime = 1_800_000_000
	token.Status = common.TokenStatusDisabled
	require.NoError(t, db.Save(token).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/?group_only=true", map[string]any{
		"id": token.Id, "group": "vip",
	}, 1)
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var stored model.Token
	require.NoError(t, db.First(&stored, token.Id).Error)
	require.Equal(t, "old-name", stored.Name)
	require.Equal(t, "vip", stored.Group)
	require.Equal(t, 900, stored.RemainQuota)
	require.False(t, stored.UnlimitedQuota)
	require.Equal(t, int64(1_800_000_000), stored.ExpiredTime)
	require.Equal(t, common.TokenStatusDisabled, stored.Status)

	var auditLog model.Log
	require.NoError(t, db.Where("user_id = ? AND type = ?", 1, model.LogTypeManage).Last(&auditLog).Error)
	require.Contains(t, auditLog.Other, `"action":"token.group_update"`)
}

func TestUpdateTokenGroupOnlyRejectsUnavailableGroup(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenOwner(t, db, 1, "owner", "owner@example.com")
	previousGroups := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousGroups))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	token := seedToken(t, db, 1, "restricted-group-token", "restricted-group-key")

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/?group_only=true", map[string]any{
		"id": token.Id, "group": "vip",
	}, 1)
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	var stored model.Token
	require.NoError(t, db.First(&stored, token.Id).Error)
	require.Equal(t, "default", stored.Group)
}

func TestGetTokenKeyRequiresOwnershipAndReturnsFullKey(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "owned-token", "owner1234token5678")

	authorizedCtx, authorizedRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 1)
	authorizedCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetTokenKey(authorizedCtx)

	authorizedResponse := decodeAPIResponse(t, authorizedRecorder)
	if !authorizedResponse.Success {
		t.Fatalf("expected authorized key fetch to succeed, got message: %s", authorizedResponse.Message)
	}

	var keyData tokenKeyResponse
	if err := common.Unmarshal(authorizedResponse.Data, &keyData); err != nil {
		t.Fatalf("failed to decode token key response: %v", err)
	}
	if keyData.Key != token.GetFullKey() {
		t.Fatalf("expected full key %q, got %q", token.GetFullKey(), keyData.Key)
	}
	var revealEvent model.AuditOutboxEvent
	require.NoError(t, db.First(&revealEvent, "event_type = ?", "token.secret_reveal").Error)
	require.Equal(t, model.AuditOutboxStatusPending, revealEvent.Status)

	unauthorizedCtx, unauthorizedRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 2)
	unauthorizedCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetTokenKey(unauthorizedCtx)

	unauthorizedResponse := decodeAPIResponse(t, unauthorizedRecorder)
	if unauthorizedResponse.Success {
		t.Fatalf("expected unauthorized key fetch to fail")
	}
	if strings.Contains(unauthorizedRecorder.Body.String(), token.Key) {
		t.Fatalf("unauthorized key response leaked raw token key: %s", unauthorizedRecorder.Body.String())
	}
}

func TestGetTokenKeyReturnsNoKeyWhenAuditEnqueueFails(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "owned-token", "owner1234token5678")
	failure := errors.New("outbox unavailable")
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("token_key_test:fail_outbox", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "audit_outbox_events" {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove("token_key_test:fail_outbox") })

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetTokenKey(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.NotContains(t, recorder.Body.String(), token.GetFullKey())
}

func TestAddTokenAcceptsValidCustomKey(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenOwner(t, db, 1, "creator", "creator@example.com")

	body := map[string]any{
		"name":                 "custom-token",
		"custom_key":           "custom_key-123456",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected custom token creation to succeed, got message: %s", response.Message)
	}

	var token model.Token
	if err := db.First(&token, "user_id = ? AND name = ?", 1, "custom-token").Error; err != nil {
		t.Fatalf("failed to load custom token: %v", err)
	}
	if token.Key != "custom_key-123456" {
		t.Fatalf("expected custom key to be stored exactly, got %q", token.Key)
	}
}

func TestAddTokenRejectsCustomKeyWhenCreationIsDisabled(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenOwner(t, db, 1, "creator", "creator@example.com")
	previous := operation_setting.GetTokenSetting().AllowCustomKeys
	operation_setting.GetTokenSetting().AllowCustomKeys = false
	t.Cleanup(func() { operation_setting.GetTokenSetting().AllowCustomKeys = previous })

	body := map[string]any{
		"name":                 "blocked-custom-token",
		"custom_key":           "custom_key-123456",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	ctx.Request.Header.Set("Accept-Language", "zh-CN")
	ctx.Set(common.RequestIdKey, "custom-key-disabled")

	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Equal(t, "token_custom_key_creation_disabled", response.Code)
	require.Contains(t, response.Message, "已关闭")
	require.Equal(t, "custom-key-disabled", response.RequestID)
	var count int64
	require.NoError(t, db.Model(&model.Token{}).Where("name = ?", "blocked-custom-token").Count(&count).Error)
	require.Zero(t, count)
}

func TestAddTokenCreatesRandomKeyWhenCustomKeyCreationIsDisabled(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenOwner(t, db, 1, "creator", "creator@example.com")
	previous := operation_setting.GetTokenSetting().AllowCustomKeys
	operation_setting.GetTokenSetting().AllowCustomKeys = false
	t.Cleanup(func() { operation_setting.GetTokenSetting().AllowCustomKeys = previous })

	body := map[string]any{
		"name":                 "random-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)

	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var token model.Token
	require.NoError(t, db.First(&token, "user_id = ? AND name = ?", 1, "random-token").Error)
	require.True(t, strings.HasPrefix(token.Key, "sk-"), "generated key %q must use the automatic API key prefix", token.Key)
	require.Len(t, token.Key, 51)
}

func TestTokenLifecycleWritesStructuredAuditWithoutRawKey(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenOwner(t, db, 1, "creator", "creator@example.com")

	body := map[string]any{
		"name":                 "audited-token",
		"custom_key":           "audit_key-123456",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	createCtx, createRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(createCtx)
	require.True(t, decodeAPIResponse(t, createRecorder).Success)

	var token model.Token
	require.NoError(t, db.First(&token, "user_id = ? AND name = ?", 1, "audited-token").Error)

	keyCtx, keyRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 1)
	keyCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetTokenKey(keyCtx)
	require.True(t, decodeAPIResponse(t, keyRecorder).Success)

	updateBody := map[string]any{
		"id":                   token.Id,
		"name":                 "audited-token-updated",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}
	updateCtx, updateRecorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", updateBody, 1)
	UpdateToken(updateCtx)
	require.True(t, decodeAPIResponse(t, updateRecorder).Success)

	deleteCtx, deleteRecorder := newAuthenticatedContext(t, http.MethodDelete, "/api/token/"+strconv.Itoa(token.Id), nil, 1)
	deleteCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	DeleteToken(deleteCtx)
	require.True(t, decodeAPIResponse(t, deleteRecorder).Success)
	delivered, failed, err := service.DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)

	var logs []model.Log
	require.NoError(t, db.Order("id asc").Find(&logs, "user_id = ? AND type = ?", 1, model.LogTypeManage).Error)
	require.Len(t, logs, 4)
	actions := make([]string, 0, len(logs))
	for _, log := range logs {
		require.NotContains(t, log.Other, token.Key)
		var other map[string]interface{}
		require.NoError(t, common.Unmarshal([]byte(log.Other), &other))
		op := other["op"].(map[string]interface{})
		actions = append(actions, op["action"].(string))
	}
	require.ElementsMatch(t, []string{"token.create", "token.secret_reveal", "token.update", "token.delete"}, actions)
}

func TestAddTokenRejectsInvalidCustomKey(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenOwner(t, db, 1, "creator", "creator@example.com")

	body := map[string]any{
		"name":                 "bad-custom-token",
		"custom_key":           "bad key with spaces",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected invalid custom token creation to fail")
	}
	if !strings.Contains(response.Message, "only contain") {
		t.Fatalf("expected a character requirement, got %q", response.Message)
	}
	var count int64
	if err := db.Model(&model.Token{}).Where("name = ?", "bad-custom-token").Count(&count).Error; err != nil {
		t.Fatalf("failed to count tokens: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no token to be created, got %d", count)
	}
}

func TestAddTokenExplainsCustomKeyLengthRequirement(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenOwner(t, db, 1, "creator", "creator@example.com")

	body := map[string]any{
		"name":                 "short-custom-token",
		"custom_key":           "too-short",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "16")
	require.Contains(t, response.Message, "128")
}

func TestDeletedCustomTokenCanBeCreatedAgain(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenOwner(t, db, 1, "creator", "creator@example.com")
	key := "recreate_key_123456"
	existing := seedToken(t, db, 1, "deleted-custom-token", key)
	require.NoError(t, db.Model(&model.Token{}).Where("id = ?", existing.Id).Updates(map[string]any{
		"key_fingerprint": "historical-fingerprint",
		"key_ciphertext":  "historical-ciphertext",
		"key_prefix":      "recr",
		"key_suffix":      "3456",
	}).Error)

	deleteCtx, deleteRecorder := newAuthenticatedContext(t, http.MethodDelete, "/api/token/"+strconv.Itoa(existing.Id), nil, 1)
	deleteCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(existing.Id)}}
	DeleteToken(deleteCtx)
	require.True(t, decodeAPIResponse(t, deleteRecorder).Success)

	createCtx, createRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", map[string]any{
		"name": "replacement-custom-token", "custom_key": key, "expired_time": -1,
		"remain_quota": 100, "unlimited_quota": true, "group": "default",
	}, 1)
	AddToken(createCtx)
	require.True(t, decodeAPIResponse(t, createRecorder).Success)

	var deleted model.Token
	require.NoError(t, db.Unscoped().First(&deleted, existing.Id).Error)
	require.True(t, deleted.DeletedAt.Valid)
	require.NotEmpty(t, deleted.Key)
	require.NotContains(t, deleted.Key, key)
	require.Empty(t, deleted.KeyFingerprint)
	require.Empty(t, deleted.KeyCiphertext)
	require.Empty(t, deleted.KeyPrefix)
	require.Empty(t, deleted.KeySuffix)
}

func TestHistoricalDeletedCustomTokenCanBeCreatedAgain(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenOwner(t, db, 1, "creator", "creator@example.com")
	key := "historical_key_12345"
	existing := seedToken(t, db, 1, "historical-deleted-token", key)
	require.NoError(t, db.Delete(existing).Error)

	createCtx, createRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", map[string]any{
		"name": "replacement-historical-token", "custom_key": key, "expired_time": -1,
		"remain_quota": 100, "unlimited_quota": true, "group": "default",
	}, 1)
	AddToken(createCtx)
	require.True(t, decodeAPIResponse(t, createRecorder).Success)

	var deleted model.Token
	require.NoError(t, db.Unscoped().First(&deleted, existing.Id).Error)
	require.NotEmpty(t, deleted.Key)
	require.NotContains(t, deleted.Key, key)
}

func TestAddTokenCollisionDisablesExistingTokenAndNotifiesOwner(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	configureTokenSecretManagerForControllerTest(t)
	owner := seedTokenOwner(t, db, 2, "owner", "owner@example.com")
	seedTokenOwner(t, db, 1, "attacker", "attacker@example.com")
	existing := seedToken(t, db, owner.Id, "existing-token", "collide_key-123456")

	var sent []struct {
		subject  string
		receiver string
		content  string
	}
	previousSender := tokenCompromiseEmailSender
	tokenCompromiseEmailSender = func(subject string, receiver string, content string) error {
		sent = append(sent, struct {
			subject  string
			receiver string
			content  string
		}{subject: subject, receiver: receiver, content: content})
		return nil
	}
	t.Cleanup(func() { tokenCompromiseEmailSender = previousSender })

	body := map[string]any{
		"name":                 "colliding-token",
		"custom_key":           "collide_key-123456",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	ctx.Request.RemoteAddr = "203.0.113.9:12345"
	ctx.Set(common.RequestIdKey, "custom-key-collision")
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected colliding custom token creation to fail")
	}
	if response.Message != customTokenUnavailableMessage {
		t.Fatalf("expected generic custom key failure, got %q", response.Message)
	}
	require.Equal(t, "token_custom_key_unavailable", response.Code)
	require.Equal(t, "custom-key-collision", response.RequestID)
	if strings.Contains(recorder.Body.String(), existing.Key) {
		t.Fatalf("collision response leaked raw token key: %s", recorder.Body.String())
	}

	var got model.Token
	if err := db.First(&got, existing.Id).Error; err != nil {
		t.Fatalf("failed to reload collided token: %v", err)
	}
	if got.Status != common.TokenStatusDisabled {
		t.Fatalf("expected collided token to be disabled, got status %d", got.Status)
	}
	if got.StatusReason != tokenCompromisedStatusReason {
		t.Fatalf("expected compromised status reason, got %q", got.StatusReason)
	}

	if len(sent) != 1 {
		t.Fatalf("expected one owner email, got %d", len(sent))
	}
	if sent[0].receiver != owner.Email {
		t.Fatalf("expected email to owner %q, got %q", owner.Email, sent[0].receiver)
	}
	if strings.Contains(sent[0].content, existing.Key) {
		t.Fatalf("owner email leaked raw token key: %s", sent[0].content)
	}
	if !strings.Contains(sent[0].content, existing.GetMaskedKey()) {
		t.Fatalf("owner email should include masked key %q, got %s", existing.GetMaskedKey(), sent[0].content)
	}

	delivered, failed, err := service.DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	var auditLog model.Log
	if err := db.First(&auditLog, "type = ?", model.LogTypeManage).Error; err != nil {
		t.Fatalf("expected collision audit log: %v", err)
	}
	if !strings.Contains(auditLog.Other, "token.custom_key_collision_suspected_compromise") {
		t.Fatalf("expected collision audit action, got %s", auditLog.Other)
	}
	if strings.Contains(auditLog.Other, existing.Key) {
		t.Fatalf("audit log leaked raw token key: %s", auditLog.Other)
	}
}

func TestAddTokenCollisionQuarantinesExistingTokenDuringSchemaPhase(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, service.ConfigureTokenSecretManager(common.TokenSecretConfig{Phase: common.TokenSecretPhaseSchema}))
	owner := seedTokenOwner(t, db, 2, "schema-owner", "")
	seedTokenOwner(t, db, 1, "schema-requester", "")
	existing := seedToken(t, db, owner.Id, "schema-existing-token", "schema_collision_key")

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", map[string]any{
		"name": "schema-collision-attempt", "custom_key": existing.Key, "expired_time": -1,
		"remain_quota": 100, "unlimited_quota": true, "group": "default",
	}, 1)
	ctx.Set(common.RequestIdKey, "schema-collision")
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Equal(t, "token_custom_key_unavailable", response.Code)
	require.Equal(t, customTokenUnavailableMessage, response.Message)
	require.Equal(t, "schema-collision", response.RequestID)
	require.NotContains(t, recorder.Body.String(), "token secret is unavailable")

	var stored model.Token
	require.NoError(t, db.First(&stored, existing.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, stored.Status)
	require.Equal(t, tokenCompromisedStatusReason, stored.StatusReason)
	require.Equal(t, "quarantined", stored.CompromiseState)
	require.NotZero(t, stored.CompromisedAt)
}

func TestAddTokenRejectsPreviouslyCompromisedCustomKeyAfterOwnerDeletesIt(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	configureTokenSecretManagerForControllerTest(t)

	owner := seedTokenOwner(t, db, 2, "owner", "owner@example.com")
	seedTokenOwner(t, db, 1, "requester", "requester@example.com")
	key := "compromised_key_123456"
	existing := seedToken(t, db, owner.Id, "compromised-token", key)
	previousSender := tokenCompromiseEmailSender
	tokenCompromiseEmailSender = func(string, string, string) error { return nil }
	t.Cleanup(func() { tokenCompromiseEmailSender = previousSender })

	firstCtx, firstRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", map[string]any{
		"name": "collision-attempt", "custom_key": key, "expired_time": -1,
		"remain_quota": 100, "unlimited_quota": true, "group": "default",
	}, 1)
	AddToken(firstCtx)
	require.False(t, decodeAPIResponse(t, firstRecorder).Success)

	manager, err := service.CurrentTokenSecretManager()
	require.NoError(t, err)
	fingerprint, err := manager.Fingerprint(key)
	require.NoError(t, err)
	revoked, err := model.IsTokenFingerprintRevoked(db, fingerprint)
	require.NoError(t, err)
	require.True(t, revoked)

	deleteCtx, deleteRecorder := newAuthenticatedContext(t, http.MethodDelete, "/api/token/"+strconv.Itoa(existing.Id), nil, owner.Id)
	deleteCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(existing.Id)}}
	DeleteToken(deleteCtx)
	require.True(t, decodeAPIResponse(t, deleteRecorder).Success)

	secondCtx, secondRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", map[string]any{
		"name": "revoked-attempt", "custom_key": key, "expired_time": -1,
		"remain_quota": 100, "unlimited_quota": true, "group": "default",
	}, 1)
	secondCtx.Set(common.RequestIdKey, "revoked-key-request")
	AddToken(secondCtx)
	secondResponse := decodeAPIResponse(t, secondRecorder)
	require.False(t, secondResponse.Success)
	require.Equal(t, "token_key_revoked", secondResponse.Code)
	require.Equal(t, "revoked-key-request", secondResponse.RequestID)

	var createdCount int64
	require.NoError(t, db.Model(&model.Token{}).Where("name = ?", "revoked-attempt").Count(&createdCount).Error)
	require.Zero(t, createdCount)
}

func TestAddTokenCollisionRollsBackDisableWhenAuditEnqueueFails(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	configureTokenSecretManagerForControllerTest(t)
	owner := seedTokenOwner(t, db, 2, "owner", "owner@example.com")
	seedTokenOwner(t, db, 1, "requester", "requester@example.com")
	existing := seedToken(t, db, owner.Id, "existing-token", "rollback_collision_key")
	failure := errors.New("outbox unavailable")
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("token_collision_test:fail_outbox", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "audit_outbox_events" {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove("token_collision_test:fail_outbox") })

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", map[string]any{
		"name": "colliding-token", "custom_key": "rollback_collision_key", "expired_time": -1,
		"remain_quota": 100, "unlimited_quota": true, "group": "default",
	}, 1)
	AddToken(ctx)

	require.False(t, decodeAPIResponse(t, recorder).Success)
	var stored model.Token
	require.NoError(t, db.First(&stored, existing.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, stored.Status)
	require.Empty(t, stored.StatusReason)
}

func TestAddTokenCollisionDoesNotQuarantineOwnersExistingToken(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	owner := seedTokenOwner(t, db, 1, "owner", "owner@example.com")
	existing := seedToken(t, db, owner.Id, "existing-token", "owners_existing_key")

	sendCount := 0
	previousSender := tokenCompromiseEmailSender
	tokenCompromiseEmailSender = func(string, string, string) error {
		sendCount++
		return nil
	}
	t.Cleanup(func() { tokenCompromiseEmailSender = previousSender })

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", map[string]any{
		"name": "duplicate-token", "custom_key": existing.Key, "expired_time": -1,
		"remain_quota": 100, "unlimited_quota": true, "group": "default",
	}, owner.Id)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Equal(t, customTokenUnavailableMessage, response.Message)
	require.Zero(t, sendCount)

	var stored model.Token
	require.NoError(t, db.First(&stored, existing.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, stored.Status)
	require.Empty(t, stored.StatusReason)
}

func TestAddTokenCollisionDetectsCredentialAliases(t *testing.T) {
	tests := []struct {
		name        string
		existingKey string
		customKey   string
	}{
		{
			name:        "sk prefix alias",
			existingKey: "prefix_alias_key_123456",
			customKey:   "sk-prefix_alias_key_123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTokenControllerTestDB(t)
			configureTokenSecretManagerForControllerTest(t)
			owner := seedTokenOwner(t, db, 2, "owner", "owner@example.com")
			seedTokenOwner(t, db, 1, "creator", "creator@example.com")
			existing := seedToken(t, db, owner.Id, "existing-token", tt.existingKey)

			var receivers []string
			previousSender := tokenCompromiseEmailSender
			tokenCompromiseEmailSender = func(subject string, receiver string, content string) error {
				receivers = append(receivers, receiver)
				return nil
			}
			t.Cleanup(func() { tokenCompromiseEmailSender = previousSender })

			body := map[string]any{
				"name":                 "colliding-token",
				"custom_key":           tt.customKey,
				"expired_time":         -1,
				"remain_quota":         100,
				"unlimited_quota":      true,
				"model_limits_enabled": false,
				"model_limits":         "",
				"group":                "default",
				"cross_group_retry":    false,
			}

			ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
			AddToken(ctx)

			response := decodeAPIResponse(t, recorder)
			require.False(t, response.Success)
			require.Equal(t, customTokenUnavailableMessage, response.Message)
			require.Equal(t, []string{owner.Email}, receivers)

			var got model.Token
			require.NoError(t, db.First(&got, existing.Id).Error)
			require.Equal(t, common.TokenStatusDisabled, got.Status)
			require.Equal(t, tokenCompromisedStatusReason, got.StatusReason)
			require.Equal(t, "quarantined", got.CompromiseState)
			require.NotZero(t, got.CompromisedAt)
			require.NotZero(t, got.CompromiseNoticeAt)

			var createdCount int64
			require.NoError(t, db.Model(&model.Token{}).Where("name = ?", "colliding-token").Count(&createdCount).Error)
			require.Zero(t, createdCount)
		})
	}
}

func TestAddTokenCollisionDoesNotTruncateCustomKeyAtHyphen(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	owner := seedTokenOwner(t, db, 2, "owner", "owner@example.com")
	seedTokenOwner(t, db, 1, "creator", "creator@example.com")
	existing := seedToken(t, db, owner.Id, "existing-token", "channelalias123456")

	body := map[string]any{
		"name": "distinct-token", "custom_key": "channelalias123456-17", "expired_time": -1,
		"remain_quota": 100, "unlimited_quota": true, "group": "default",
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)
	require.True(t, decodeAPIResponse(t, recorder).Success)

	var unchanged model.Token
	require.NoError(t, db.First(&unchanged, existing.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, unchanged.Status)
	var created model.Token
	require.NoError(t, db.First(&created, "user_id = ? AND name = ?", 1, "distinct-token").Error)
	require.Equal(t, "channelalias123456-17", created.Key)
}

func TestAddTokenCollisionIsIdempotentForAlreadyDisabledToken(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	owner := seedTokenOwner(t, db, 2, "owner", "owner@example.com")
	seedTokenOwner(t, db, 1, "attacker", "attacker@example.com")
	existing := seedToken(t, db, owner.Id, "existing-token", "disabled_collision_key")
	if err := db.Model(existing).Update("status", common.TokenStatusDisabled).Error; err != nil {
		t.Fatalf("failed to disable token: %v", err)
	}

	sendCount := 0
	previousSender := tokenCompromiseEmailSender
	tokenCompromiseEmailSender = func(subject string, receiver string, content string) error {
		sendCount++
		return nil
	}
	t.Cleanup(func() { tokenCompromiseEmailSender = previousSender })

	body := map[string]any{
		"name":                 "colliding-token",
		"custom_key":           "disabled_collision_key",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected colliding custom token creation to fail")
	}
	if sendCount != 0 {
		t.Fatalf("expected no duplicate email for already disabled token, got %d", sendCount)
	}
	var auditCount int64
	if err := db.Model(&model.Log{}).Where("type = ?", model.LogTypeManage).Count(&auditCount).Error; err != nil {
		t.Fatalf("failed to count audit logs: %v", err)
	}
	if auditCount != 0 {
		t.Fatalf("expected no duplicate audit log for already disabled token, got %d", auditCount)
	}
}

func TestUpdateTokenRejectsReEnableForCompromisedToken(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenOwner(t, db, 1, "owner", "owner@example.com")
	token := seedToken(t, db, 1, "compromised-token", "compromised_key_123")
	require.NoError(t, db.Model(token).Updates(map[string]any{
		"status":        common.TokenStatusDisabled,
		"status_reason": tokenCompromisedStatusReason,
	}).Error)

	body := map[string]any{
		"id":     token.Id,
		"status": common.TokenStatusEnabled,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/?status_only=true", body, 1)
	ctx.Request.URL.RawQuery = "status_only=true"
	ctx.Set(common.RequestIdKey, "compromised-reenable")
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected compromised token re-enable to fail")
	}
	if response.Message != tokenCompromisedReEnableMessage {
		t.Fatalf("expected compromised re-enable message, got %q", response.Message)
	}
	require.Equal(t, "token_compromised_reenable_forbidden", response.Code)
	require.Equal(t, "compromised-reenable", response.RequestID)

	var got model.Token
	require.NoError(t, db.First(&got, token.Id).Error)
	if got.Status != common.TokenStatusDisabled {
		t.Fatalf("expected compromised token to stay disabled, got %d", got.Status)
	}
}

func TestGetTokenReturnsCompromisedStatusReason(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenOwner(t, db, 1, "owner", "owner@example.com")
	token := seedToken(t, db, 1, "compromised-token", "compromised_key_456")
	require.NoError(t, db.Model(token).Updates(map[string]any{
		"status":        common.TokenStatusDisabled,
		"status_reason": tokenCompromisedStatusReason,
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/"+strconv.Itoa(token.Id), nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetToken(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success)
	var detail tokenResponseItem
	require.NoError(t, common.Unmarshal(response.Data, &detail))
	if detail.StatusReason != tokenCompromisedStatusReason {
		t.Fatalf("expected compromised status reason, got %q", detail.StatusReason)
	}
}

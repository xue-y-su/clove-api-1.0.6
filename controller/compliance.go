package controller

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/compliance_setting"
	"github.com/gin-gonic/gin"
)

type complianceStatementResponse struct {
	Enabled     bool                         `json:"enabled"`
	Version     string                       `json:"version"`
	Content     string                       `json:"content"`
	ContentHash string                       `json:"content_hash"`
	Confirmed   bool                         `json:"confirmed"`
	Bypassed    bool                         `json:"bypassed,omitempty"`
	DisplayMode string                       `json:"display_mode"`
	Snapshots   []complianceSnapshotResponse `json:"snapshots"`
}

type complianceSettingsResponse struct {
	StatementEnabled           bool                                 `json:"statement_enabled"`
	StatementVersion           string                               `json:"statement_version"`
	StatementContent           string                               `json:"statement_content"`
	StatementItems             []compliance_setting.StatementItem   `json:"statement_items"`
	StatementDisplayMode       string                               `json:"statement_display_mode"`
	TokenAPIEnforcementEnabled bool                                 `json:"token_api_enforcement_enabled"`
	ShowVersionToUser          bool                                 `json:"show_version_to_user"`
	ShowContentHashToUser      bool                                 `json:"show_content_hash_to_user"`
	ContentHash                string                               `json:"content_hash"`
	Snapshots                  []complianceSettingsSnapshotResponse `json:"snapshots,omitempty"`
}

type complianceSnapshotResponse struct {
	Id                 int      `json:"id"`
	ItemKey            string   `json:"item_key"`
	Title              string   `json:"title"`
	SourceType         string   `json:"source_type"`
	Version            string   `json:"version"`
	Content            string   `json:"content"`
	ContentHash        string   `json:"content_hash"`
	ContentLength      int      `json:"content_length"`
	SortOrder          int      `json:"sort_order"`
	ConfirmationMode   string   `json:"confirmation_mode"`
	ConfirmationPhrase string   `json:"confirmation_phrase,omitempty"`
	Segments           []string `json:"segments,omitempty"`
	Confirmed          bool     `json:"confirmed"`
}

type complianceSettingsSnapshotResponse struct {
	Id            int    `json:"id"`
	ItemKey       string `json:"item_key"`
	Version       string `json:"version"`
	ContentHash   string `json:"content_hash"`
	ContentLength int    `json:"content_length"`
	SortOrder     int    `json:"sort_order"`
	CreatedAt     int64  `json:"created_at"`
}

type complianceConfirmRequest struct {
	SnapshotId        int      `json:"snapshot_id"`
	ConfirmationText  string   `json:"confirmation_text"`
	ConfirmationTexts []string `json:"confirmation_texts"`
	Segments          []string `json:"segments"`
}

func GetComplianceStatement(c *gin.Context) {
	settings := compliance_setting.GetSettings()
	confirmed := !settings.StatementEnabled
	bypassed := false
	snapshotResponses := make([]complianceSnapshotResponse, 0)
	userId := c.GetInt("id")
	if settings.StatementEnabled && userId > 0 && model.IsRootUser(userId) {
		confirmed = true
		bypassed = true
	}
	if settings.StatementEnabled && userId > 0 && !bypassed && compliance_setting.ValidateSettings(settings) == nil {
		snapshots, err := model.EnsureComplianceStatementSnapshots(settings)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		confirmed, err = model.HasAllComplianceSnapshotConfirmations(userId, snapshots)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		for _, snapshot := range snapshots {
			snapshotConfirmed, err := isComplianceSnapshotConfirmed(userId, snapshot)
			if err != nil {
				common.ApiError(c, err)
				return
			}
			snapshotResponses = append(snapshotResponses, buildComplianceSnapshotResponse(snapshot, snapshotConfirmed))
		}
		if !confirmed && len(snapshots) == 1 && snapshotResponses[0].Confirmed {
			confirmed = true
		}
	}
	hash := compliance_setting.ContentHash(settings.StatementContent)
	response := complianceStatementResponse{
		Enabled:     settings.StatementEnabled,
		Version:     settings.StatementVersion,
		Content:     settings.StatementContent,
		ContentHash: hash,
		Confirmed:   confirmed,
		Bypassed:    bypassed,
		DisplayMode: settings.StatementDisplayMode,
		Snapshots:   snapshotResponses,
	}
	applyComplianceDisplayVisibility(&response, settings, bypassed)
	common.ApiSuccess(c, response)
}

func ConfirmComplianceStatement(c *gin.Context) {
	settings := compliance_setting.GetSettings()
	if !settings.StatementEnabled {
		common.ApiSuccess(c, gin.H{"confirmed": true})
		return
	}
	if err := compliance_setting.ValidateSettings(settings); err != nil {
		common.ApiError(c, err)
		return
	}
	snapshots, err := model.EnsureComplianceStatementSnapshots(settings)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(snapshots) == 0 {
		common.ApiSuccess(c, gin.H{"confirmed": true})
		return
	}

	request, err := parseComplianceConfirmRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	userId := c.GetInt("id")
	username := c.GetString("username")
	confirmedSnapshots := make([]complianceSnapshotResponse, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if request.SnapshotId > 0 && snapshot.Id != request.SnapshotId {
			continue
		}
		if err := validateComplianceConfirmation(snapshot, request); err != nil {
			common.ApiError(c, err)
			return
		}
		if err := model.RecordComplianceSnapshotConfirmation(userId, username, snapshot, c.ClientIP(), c.Request.UserAgent(), common.GetTimestamp()); err != nil {
			common.ApiError(c, err)
			return
		}
		confirmedSnapshots = append(confirmedSnapshots, buildComplianceSnapshotResponse(snapshot, true))
	}
	if request.SnapshotId > 0 && len(confirmedSnapshots) == 0 {
		common.ApiError(c, errors.New("合规声明快照不存在"))
		return
	}
	middleware.ClearComplianceConfirmationCache()
	recordUserSecurityAudit(c, userId, "compliance.confirm", map[string]interface{}{
		"version":      settings.StatementVersion,
		"content_hash": compliance_setting.ContentHash(settings.StatementContent),
		"snapshot_id":  request.SnapshotId,
		"count":        len(confirmedSnapshots),
	})
	overallConfirmed, err := model.HasAllComplianceSnapshotConfirmations(userId, snapshots)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	applyComplianceSnapshotDisplayVisibility(confirmedSnapshots, settings, userId > 0 && model.IsRootUser(userId))
	common.ApiSuccess(c, gin.H{
		"confirmed": overallConfirmed,
		"snapshots": confirmedSnapshots,
	})
}

func GetComplianceSettings(c *gin.Context) {
	common.ApiSuccess(c, buildComplianceSettingsResponse(compliance_setting.GetSettings()))
}

func UpdateComplianceSettings(c *gin.Context) {
	var request compliance_setting.Settings
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}
	request = compliance_setting.NormalizeSettings(request)
	if err := compliance_setting.ValidateSettings(request); err != nil {
		common.ApiError(c, err)
		return
	}

	oldSettings := compliance_setting.GetSettings()
	oldHash := compliance_setting.ContentHash(oldSettings.StatementContent)
	newHash := compliance_setting.ContentHash(request.StatementContent)
	if err := model.UpdateOptionsBulk(compliance_setting.SettingsToOptions(request)); err != nil {
		common.ApiError(c, err)
		return
	}
	snapshots, err := model.EnsureComplianceStatementSnapshots(request)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	middleware.ClearComplianceConfirmationCache()
	recordManageAudit(c, "compliance.settings_update", map[string]interface{}{
		"statement_enabled_from":             oldSettings.StatementEnabled,
		"statement_enabled_to":               request.StatementEnabled,
		"token_api_enforcement_enabled_from": oldSettings.TokenAPIEnforcementEnabled,
		"token_api_enforcement_enabled_to":   request.TokenAPIEnforcementEnabled,
		"version_from":                       oldSettings.StatementVersion,
		"version_to":                         request.StatementVersion,
		"content_hash_from":                  oldHash,
		"content_hash_to":                    newHash,
		"content_length_from":                len(oldSettings.StatementContent),
		"content_length_to":                  len(request.StatementContent),
		"statement_items_from":               len(compliance_setting.EffectiveStatementItems(oldSettings)),
		"statement_items_to":                 len(compliance_setting.EffectiveStatementItems(request)),
		"show_version_to_user_from":          oldSettings.ShowVersionToUser,
		"show_version_to_user_to":            request.ShowVersionToUser,
		"show_content_hash_to_user_from":     oldSettings.ShowContentHashToUser,
		"show_content_hash_to_user_to":       request.ShowContentHashToUser,
	})
	common.ApiSuccess(c, buildComplianceSettingsResponse(request, snapshots))
}

func GetComplianceConfirmations(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	ipFilter, err := scopedSensitiveIPFilter(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filter := model.ComplianceConfirmationFilter{
		UserId:         parsePositiveIntQuery(c, "user_id"),
		SnapshotId:     parsePositiveIntQuery(c, "snapshot_id"),
		Username:       c.Query("username"),
		Version:        c.Query("version"),
		ContentHash:    c.Query("content_hash"),
		Ip:             ipFilter,
		StartTimestamp: int64(parsePositiveIntQuery(c, "start_timestamp")),
		EndTimestamp:   int64(parsePositiveIntQuery(c, "end_timestamp")),
	}
	items, total, err := model.GetComplianceConfirmations(filter, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	if err := projectSensitiveIPList(c, pageInfo, "compliance-confirmations", "compliance.full_ip_read", len(items), func() {
		for _, item := range items {
			item.Ip = model.MaskAuditIP(item.Ip)
		}
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, pageInfo)
}

func buildComplianceSettingsResponse(settings compliance_setting.Settings, snapshots ...[]model.ComplianceStatementSnapshot) complianceSettingsResponse {
	snapshotResponses := make([]complianceSettingsSnapshotResponse, 0)
	if len(snapshots) > 0 {
		for _, snapshot := range snapshots[0] {
			snapshotResponses = append(snapshotResponses, complianceSettingsSnapshotResponse{
				Id:            snapshot.Id,
				ItemKey:       snapshot.ItemKey,
				Version:       snapshot.Version,
				ContentHash:   snapshot.ContentHash,
				ContentLength: snapshot.ContentLength,
				SortOrder:     snapshot.SortOrder,
				CreatedAt:     snapshot.CreatedAt,
			})
		}
	}
	return complianceSettingsResponse{
		StatementEnabled:           settings.StatementEnabled,
		StatementVersion:           settings.StatementVersion,
		StatementContent:           settings.StatementContent,
		StatementItems:             settings.StatementItems,
		StatementDisplayMode:       settings.StatementDisplayMode,
		TokenAPIEnforcementEnabled: settings.TokenAPIEnforcementEnabled,
		ShowVersionToUser:          settings.ShowVersionToUser,
		ShowContentHashToUser:      settings.ShowContentHashToUser,
		ContentHash:                compliance_setting.ContentHash(settings.StatementContent),
		Snapshots:                  snapshotResponses,
	}
}

func applyComplianceDisplayVisibility(response *complianceStatementResponse, settings compliance_setting.Settings, bypassed bool) {
	showVersion := settings.ShowVersionToUser || bypassed
	showContentHash := settings.ShowContentHashToUser || bypassed
	if !showVersion {
		response.Version = ""
	}
	if !showContentHash {
		response.ContentHash = ""
	}
	for i := range response.Snapshots {
		if !showVersion {
			response.Snapshots[i].Version = ""
		}
		if !showContentHash {
			response.Snapshots[i].ContentHash = ""
		}
	}
}

func applyComplianceSnapshotDisplayVisibility(snapshots []complianceSnapshotResponse, settings compliance_setting.Settings, bypassed bool) {
	showVersion := settings.ShowVersionToUser || bypassed
	showContentHash := settings.ShowContentHashToUser || bypassed
	for i := range snapshots {
		if !showVersion {
			snapshots[i].Version = ""
		}
		if !showContentHash {
			snapshots[i].ContentHash = ""
		}
	}
}

func parseComplianceConfirmRequest(c *gin.Context) (complianceConfirmRequest, error) {
	var request complianceConfirmRequest
	if c.Request == nil || c.Request.Body == nil {
		return request, nil
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return request, err
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return request, nil
	}
	if err := common.Unmarshal(body, &request); err != nil {
		return request, err
	}
	return request, nil
}

func validateComplianceConfirmation(snapshot model.ComplianceStatementSnapshot, request complianceConfirmRequest) error {
	switch snapshot.ConfirmationMode {
	case "", compliance_setting.ConfirmationModeCheckbox:
		return nil
	case compliance_setting.ConfirmationModePhrase:
		if strings.TrimSpace(request.ConfirmationText) == strings.TrimSpace(snapshot.ConfirmationPhrase) {
			return nil
		}
		for _, text := range request.ConfirmationTexts {
			if strings.TrimSpace(text) == strings.TrimSpace(snapshot.ConfirmationPhrase) {
				return nil
			}
		}
		return errors.New("确认短语不匹配")
	case compliance_setting.ConfirmationModeSegments:
		var expected []string
		if err := common.UnmarshalJsonStr(snapshot.Segments, &expected); err != nil {
			return err
		}
		if len(request.Segments) != len(expected) {
			return errors.New("分段确认内容不完整")
		}
		for i := range expected {
			if strings.TrimSpace(request.Segments[i]) != strings.TrimSpace(expected[i]) {
				return errors.New("分段确认内容不匹配")
			}
		}
		return nil
	default:
		return errors.New("无效的合规确认方式")
	}
}

func isComplianceSnapshotConfirmed(userId int, snapshot model.ComplianceStatementSnapshot) (bool, error) {
	confirmed, err := model.HasAllComplianceSnapshotConfirmations(userId, []model.ComplianceStatementSnapshot{snapshot})
	if err != nil || confirmed {
		return confirmed, err
	}
	return model.HasComplianceConfirmation(userId, snapshot.Version, snapshot.ContentHash)
}

func buildComplianceSnapshotResponse(snapshot model.ComplianceStatementSnapshot, confirmed bool) complianceSnapshotResponse {
	var segments []string
	_ = common.UnmarshalJsonStr(snapshot.Segments, &segments)
	return complianceSnapshotResponse{
		Id:                 snapshot.Id,
		ItemKey:            snapshot.ItemKey,
		Title:              snapshot.Title,
		SourceType:         snapshot.SourceType,
		Version:            snapshot.Version,
		Content:            snapshot.Content,
		ContentHash:        snapshot.ContentHash,
		ContentLength:      snapshot.ContentLength,
		SortOrder:          snapshot.SortOrder,
		ConfirmationMode:   snapshot.ConfirmationMode,
		ConfirmationPhrase: snapshot.ConfirmationPhrase,
		Segments:           segments,
		Confirmed:          confirmed,
	}
}

func parsePositiveIntQuery(c *gin.Context, key string) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil || value < 0 {
		return 0
	}
	return value
}

package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type tokenAutoGroupsInput struct {
	Set    bool
	Groups []string
}

func (input *tokenAutoGroupsInput) UnmarshalJSON(data []byte) error {
	input.Set = true
	if strings.TrimSpace(string(data)) == "null" {
		input.Groups = nil
		return nil
	}
	return common.Unmarshal(data, &input.Groups)
}

type tokenRequest struct {
	model.Token
	CustomKey  string               `json:"custom_key"`
	AutoGroups tokenAutoGroupsInput `json:"auto_groups"`
}

type tokenResponse struct {
	*model.Token
	AutoGroups []string `json:"auto_groups"`
}

func buildMaskedTokenResponse(token *model.Token) *tokenResponse {
	if token == nil {
		return nil
	}
	maskedToken := *token
	maskedToken.Key = token.GetMaskedKey()
	autoGroups, err := token.GetAutoGroups()
	if err != nil {
		common.SysError(fmt.Sprintf("failed to parse auto groups for token %d: %v", token.Id, err))
		autoGroups = nil
	}
	if len(autoGroups) == 0 {
		autoGroups = nil
	}
	return &tokenResponse{Token: &maskedToken, AutoGroups: autoGroups}
}

func buildMaskedTokenResponses(tokens []*model.Token) []*tokenResponse {
	maskedTokens := make([]*tokenResponse, 0, len(tokens))
	for _, token := range tokens {
		maskedTokens = append(maskedTokens, buildMaskedTokenResponse(token))
	}
	return maskedTokens
}

func recordTokenAudit(c *gin.Context, action string, token *model.Token, extra map[string]interface{}) {
	if token == nil {
		return
	}
	params := map[string]interface{}{
		"token_id":   token.Id,
		"name":       token.Name,
		"masked_key": token.GetMaskedKey(),
		"status":     token.Status,
		"group":      token.Group,
	}
	for key, value := range extra {
		params[key] = value
	}
	recordUserSecurityAudit(c, token.UserId, action, params)
}

const customTokenUnavailableMessage = "自定义 API Key 不可用，请更换后重试。"
const tokenCompromisedStatusReason = "custom_key_collision_suspected_compromise"
const tokenCompromisedReEnableMessage = "此 API Key 因疑似泄露已被系统禁用，请创建并替换新的 Key。"
const tokenKeyRevokedMessage = "该自定义 API Key 曾被标记为疑似泄露，不能再次使用。"

var errCustomTokenKeyRevoked = errors.New("token_key_revoked")

var tokenCompromiseEmailSender = common.SendEmail

func getTokenRequestUserGroup(c *gin.Context) (string, error) {
	if userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup); userGroup != "" {
		return userGroup, nil
	}
	if userGroup := c.GetString("group"); userGroup != "" {
		return userGroup, nil
	}
	return model.GetUserGroup(c.GetInt("id"), false)
}

func setTokenAutoGroups(c *gin.Context, token *model.Token, groups []string) bool {
	if len(groups) == 0 {
		if err := token.SetAutoGroups(nil); err != nil {
			common.ApiError(c, err)
			return false
		}
		return true
	}

	maxCount := setting.GetMaxTokenAutoGroups()
	if len(groups) > maxCount {
		common.ApiErrorI18n(c, i18n.MsgTokenAutoGroupsTooMany, map[string]any{"Max": maxCount})
		return false
	}

	userGroup, err := getTokenRequestUserGroup(c)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if _, ok := seen[group]; ok {
			common.ApiErrorI18n(c, i18n.MsgTokenAutoGroupsDuplicate, map[string]any{"Group": group})
			return false
		}
		seen[group] = struct{}{}
		if !service.IsUserSelectableGroup(userGroup, group) {
			common.ApiErrorI18n(c, i18n.MsgTokenAutoGroupsInvalid, map[string]any{"Group": group})
			return false
		}
	}

	if err := token.SetAutoGroups(groups); err != nil {
		common.ApiError(c, err)
		return false
	}
	return true
}

const (
	customTokenKeyMinLength = 16
	customTokenKeyMaxLength = 128
)

type customTokenKeyValidation int

const (
	customTokenKeyValid customTokenKeyValidation = iota
	customTokenKeyInvalidLength
	customTokenKeyInvalidCharacters
)

func validateCustomTokenKey(key string) customTokenKeyValidation {
	if len(key) < customTokenKeyMinLength || len(key) > customTokenKeyMaxLength {
		return customTokenKeyInvalidLength
	}
	for _, r := range key {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '_' || r == '-' {
			continue
		}
		return customTokenKeyInvalidCharacters
	}
	return customTokenKeyValid

}

func sendTokenCompromiseEmail(owner *model.User, token *model.Token) error {
	if owner == nil || token == nil || strings.TrimSpace(owner.Email) == "" {
		return nil
	}
	maskedKey := token.GetMaskedKey()
	subject := "Clove API Key 安全提醒"
	content := fmt.Sprintf("<p>你的 API Key（%s）因疑似泄露已被系统禁用。</p><p>请在控制台创建新的 API Key，并尽快替换使用旧 Key 的应用配置。</p>", maskedKey)
	return tokenCompromiseEmailSender(subject, owner.Email, content)
}

func customTokenCollisionCandidates(key string) []string {
	return model.TokenKeyLookupCandidates(key)
}

func handleCustomTokenCollision(c *gin.Context, key string) (bool, error) {
	manager, err := service.CurrentTokenSecretManager()
	if err != nil {
		return false, err
	}
	candidates := customTokenCollisionCandidates(key)
	if manager.HasKeys() {
		revoked, err := manager.IsTokenKeyRevoked(model.DB, key)
		if err != nil {
			return false, err
		}
		if revoked {
			return true, errCustomTokenKeyRevoked
		}
	}

	var existing *model.Token
	matchedKey := ""
	for _, candidate := range candidates {
		token, err := model.GetTokenByKeyNoSQLLog(candidate)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return false, err
		}
		existing = token
		matchedKey = candidate
		break
	}
	if existing == nil {
		return false, nil
	}
	if existing.UserId == c.GetInt("id") {
		return true, nil
	}
	if existing.Status != common.TokenStatusEnabled {
		if existing.StatusReason == tokenCompromisedStatusReason {
			if manager.HasKeys() {
				if err := model.DB.Transaction(func(tx *gorm.DB) error {
					revoked, err := manager.IsTokenKeyRevoked(tx, matchedKey)
					if err != nil || revoked {
						return err
					}
					return manager.RevokeTokenKey(tx, matchedKey, "compromised")
				}); err != nil {
					return true, err
				}
				return true, errCustomTokenKeyRevoked
			}
			return true, nil
		}
		return true, nil
	}

	existing.Status = common.TokenStatusDisabled
	existing.StatusReason = tokenCompromisedStatusReason
	now := common.GetTimestamp()
	quarantined := false
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Token{}).
			Where("id = ? AND status = ?", existing.Id, common.TokenStatusEnabled).
			Updates(map[string]interface{}{
				"status":           common.TokenStatusDisabled,
				"status_reason":    tokenCompromisedStatusReason,
				"compromise_state": "quarantined",
				"compromised_at":   now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		if manager.HasKeys() {
			if err := manager.RevokeTokenKey(tx, matchedKey, "compromised"); err != nil {
				return err
			}
		}
		quarantined = true
		return enqueueCriticalAuditWithTx(c, tx, c.GetInt("id"), fmt.Sprintf("token:%d", existing.Id), "token.custom_key_collision_suspected_compromise", map[string]interface{}{
			"token_id":             existing.Id,
			"token_user_id":        existing.UserId,
			"masked_key":           existing.GetMaskedKey(),
			"requested_by_user_id": c.GetInt("id"),
			"result":               "disabled",
			"reason":               "custom_key_collision",
		})
	}); err != nil {
		return true, err
	}
	if !quarantined {
		return true, nil
	}
	if err := model.InvalidateUserTokensCache(existing.UserId); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate compromised token cache, tokenId=%d userId=%d: %s", existing.Id, existing.UserId, err.Error()))
	}
	owner, ownerErr := model.GetUserById(existing.UserId, false)
	if ownerErr != nil {
		common.SysLog(fmt.Sprintf("failed to load token owner for compromise email, tokenId=%d userId=%d: %s", existing.Id, existing.UserId, ownerErr.Error()))
	} else if err := sendTokenCompromiseEmail(owner, existing); err != nil {
		common.SysLog(fmt.Sprintf("failed to send token compromise email, tokenId=%d userId=%d: %s", existing.Id, existing.UserId, err.Error()))
	} else if strings.TrimSpace(owner.Email) != "" {
		if err := model.DB.Model(&model.Token{}).
			Where("id = ? AND compromise_notice_sent_at = ?", existing.Id, 0).
			Update("compromise_notice_sent_at", common.GetTimestamp()).Error; err != nil {
			common.SysLog(fmt.Sprintf("failed to record token compromise email, tokenId=%d userId=%d: %s", existing.Id, existing.UserId, err.Error()))
		}
	}
	return true, nil
}

func GetAllTokens(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	tokens, err := model.GetAllUserTokens(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total, _ := model.CountUserTokens(userId)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(buildMaskedTokenResponses(tokens))
	common.ApiSuccess(c, pageInfo)
}

func SearchTokens(c *gin.Context) {
	userId := c.GetInt("id")
	keyword := c.Query("keyword")
	token := c.Query("token")

	pageInfo := common.GetPageQuery(c)

	tokens, total, err := service.SearchUserTokens(c.Request.Context(), userId, keyword, token, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(buildMaskedTokenResponses(tokens))
	common.ApiSuccess(c, pageInfo)
}

func GetToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.GetTokenByIds(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildMaskedTokenResponse(token))
}

func GetTokenAutoGroups(c *gin.Context) {
	userGroup, err := getTokenRequestUserGroup(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"groups":    service.GetUserAutoGroup(userGroup),
		"max_count": setting.GetMaxTokenAutoGroups(),
	})
}

func GetTokenKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.GetTokenByIds(id, userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = enqueueCriticalAudit(c, userId, fmt.Sprintf("token:%d", id), "token.secret_reveal", "rejected", map[string]interface{}{
				"token_id": id,
				"reason":   "owner_mismatch_or_missing",
			})
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "API Key 不存在或无权查看", "code": "TOKEN_NOT_FOUND"})
			return
		}
		common.ApiError(c, err)
		return
	}
	manager, err := service.CurrentTokenSecretManager()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	key, err := manager.RevealTokenKey(token)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	key = token.FormatKeyForClient(key)
	if err := enqueueCriticalAudit(c, token.UserId, fmt.Sprintf("token:%d", token.Id), "token.secret_reveal", "success", map[string]interface{}{
		"token_id":   token.Id,
		"name":       token.Name,
		"masked_key": token.GetMaskedKey(),
		"status":     token.Status,
		"group":      token.Group,
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, gin.H{
		"key": key,
	})
}

func GetTokenStatus(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	userId := c.GetInt("id")
	token, err := model.GetTokenByIds(tokenId, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	expiredAt := token.ExpiredTime
	if expiredAt == -1 {
		expiredAt = 0
	}
	c.JSON(http.StatusOK, gin.H{
		"object":          "credit_summary",
		"total_granted":   token.RemainQuota,
		"total_used":      0, // not supported currently
		"total_available": token.RemainQuota,
		"expires_at":      expiredAt * 1000,
	})
}

func GetTokenUsage(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "No Authorization header",
		})
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Invalid Bearer token",
		})
		return
	}
	tokenKey := parts[1]

	token, err := service.LookupTokenByKey(c.Request.Context(), tokenKey)
	if err != nil {
		common.SysError("failed to get token by key: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgTokenGetInfoFailed)
		return
	}

	expiredAt := token.ExpiredTime
	if expiredAt == -1 {
		expiredAt = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    true,
		"message": "ok",
		"data": gin.H{
			"object":               "token_usage",
			"name":                 token.Name,
			"total_granted":        token.RemainQuota + token.UsedQuota,
			"total_used":           token.UsedQuota,
			"total_available":      token.RemainQuota,
			"unlimited_quota":      token.UnlimitedQuota,
			"model_limits":         token.GetModelLimitsMap(),
			"model_limits_enabled": token.ModelLimitsEnabled,
			"expires_at":           expiredAt,
		},
	})
}

func AddToken(c *gin.Context) {
	request := tokenRequest{}
	err := c.ShouldBindJSON(&request)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token := request.Token
	if len(token.Name) > 50 {
		common.ApiErrorI18n(c, i18n.MsgTokenNameTooLong)
		return
	}
	// 非无限额度时，检查额度值是否超出有效范围
	if !token.UnlimitedQuota {
		if token.RemainQuota < 0 {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaNegative)
			return
		}
		maxQuotaValue := common.MaxQuota
		if token.RemainQuota > maxQuotaValue {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuotaValue})
			return
		}
	}
	// 检查用户令牌数量是否已达上限
	maxTokens := operation_setting.GetMaxUserTokens()
	count, err := model.CountUserTokens(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if int(count) >= maxTokens {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("已达到最大令牌数量限制 (%d)", maxTokens),
		})
		return
	}
	if token.Group == "auto" {
		if !setTokenAutoGroups(c, &token, request.AutoGroups.Groups) {
			return
		}
	} else {
		token.CrossGroupRetry = false
		_ = token.SetAutoGroups(nil)
	}
	key := strings.TrimSpace(request.CustomKey)
	if key != "" {
		if !operation_setting.GetTokenSetting().AllowCustomKeys {
			common.ApiErrorCodeI18n(c, "token_custom_key_creation_disabled", i18n.MsgTokenCustomKeyCreationDisabled)
			return
		}
		switch validateCustomTokenKey(key) {
		case customTokenKeyInvalidLength:
			common.ApiErrorCodeI18n(c, "token_custom_key_invalid_length", i18n.MsgTokenCustomKeyInvalidLength, map[string]any{"Min": customTokenKeyMinLength, "Max": customTokenKeyMaxLength})
			return
		case customTokenKeyInvalidCharacters:
			common.ApiErrorCodeI18n(c, "token_custom_key_invalid_characters", i18n.MsgTokenCustomKeyInvalidCharacters)
			return
		}
		collided, err := handleCustomTokenCollision(c, key)
		if err != nil {
			if errors.Is(err, errCustomTokenKeyRevoked) {
				common.ApiErrorCode(c, "token_key_revoked", tokenKeyRevokedMessage)
				return
			}
			common.ApiError(c, err)
			return
		}
		if collided {
			common.ApiErrorCode(c, "token_custom_key_unavailable", customTokenUnavailableMessage)
			return
		}
	} else {
		key, err = common.GenerateAPIKey()
		if err != nil {
			common.ApiErrorI18n(c, i18n.MsgTokenGenerateFailed)
			common.SysLog("failed to generate token key: " + err.Error())
			return
		}
	}
	cleanToken := model.Token{
		UserId:             c.GetInt("id"),
		Name:               token.Name,
		Key:                key,
		KeyFormat:          model.TokenKeyFormatFull,
		CreatedTime:        common.GetTimestamp(),
		AccessedTime:       common.GetTimestamp(),
		ExpiredTime:        token.ExpiredTime,
		RemainQuota:        token.RemainQuota,
		UnlimitedQuota:     token.UnlimitedQuota,
		ModelLimitsEnabled: token.ModelLimitsEnabled,
		ModelLimits:        token.ModelLimits,
		AllowIps:           token.AllowIps,
		Group:              token.Group,
		CrossGroupRetry:    token.CrossGroupRetry,
		AutoGroups:         token.AutoGroups,
	}
	if request.CustomKey != "" {
		if _, err := model.ReleaseDeletedTokenKeyMaterial(key); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	err = cleanToken.Insert()
	if err != nil {
		if request.CustomKey != "" {
			collided, collisionErr := handleCustomTokenCollision(c, key)
			if collisionErr != nil {
				if errors.Is(collisionErr, errCustomTokenKeyRevoked) {
					common.ApiErrorCode(c, "token_key_revoked", tokenKeyRevokedMessage)
					return
				}
				common.ApiError(c, collisionErr)
				return
			}
			if collided {
				common.ApiErrorCode(c, "token_custom_key_unavailable", customTokenUnavailableMessage)
				return
			}
		}
		common.ApiError(c, err)
		return
	}
	recordTokenAudit(c, "token.create", &cleanToken, map[string]interface{}{
		"custom_key": request.CustomKey != "",
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteToken(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	token, err := model.GetTokenByIds(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	err = model.DeleteTokenById(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordTokenAudit(c, "token.delete", token, nil)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateToken(c *gin.Context) {
	userId := c.GetInt("id")
	if c.Query("group_only") == "true" {
		var request struct {
			Id    int    `json:"id" binding:"required"`
			Group string `json:"group" binding:"required"`
		}
		if err := c.ShouldBindJSON(&request); err != nil {
			common.ApiError(c, err)
			return
		}
		userGroup, err := model.GetUserGroup(userId, false)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		request.Group = strings.TrimSpace(request.Group)
		if request.Group != "auto" && !service.IsUserSelectableGroup(userGroup, request.Group) {
			common.ApiError(c, errors.New("所选分组当前不可用，请刷新页面后重试"))
			return
		}
		cleanToken, err := model.UpdateTokenGroup(request.Id, userId, request.Group)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		recordTokenAudit(c, "token.group_update", cleanToken, nil)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    buildMaskedTokenResponse(cleanToken),
		})
		return
	}
	statusOnly := c.Query("status_only")
	request := tokenRequest{}
	err := c.ShouldBindJSON(&request)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token := request.Token
	if len(token.Name) > 50 {
		common.ApiErrorI18n(c, i18n.MsgTokenNameTooLong)
		return
	}
	if !token.UnlimitedQuota {
		if token.RemainQuota < 0 {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaNegative)
			return
		}
		maxQuotaValue := common.MaxQuota
		if token.RemainQuota > maxQuotaValue {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuotaValue})
			return
		}
	}
	cleanToken, err := model.GetTokenByIds(token.Id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if token.Status == common.TokenStatusEnabled {
		if cleanToken.StatusReason == tokenCompromisedStatusReason {
			common.ApiErrorCode(c, "token_compromised_reenable_forbidden", tokenCompromisedReEnableMessage)
			return
		}
		if cleanToken.Status == common.TokenStatusExpired && cleanToken.ExpiredTime <= common.GetTimestamp() && cleanToken.ExpiredTime != -1 {
			common.ApiErrorI18n(c, i18n.MsgTokenExpiredCannotEnable)
			return
		}
		if cleanToken.Status == common.TokenStatusExhausted && cleanToken.RemainQuota <= 0 && !cleanToken.UnlimitedQuota {
			common.ApiErrorI18n(c, i18n.MsgTokenExhaustedCannotEable)
			return
		}
	}
	if statusOnly != "" {
		cleanToken.Status = token.Status
	} else {
		// If you add more fields, please also update token.Update()
		cleanToken.Name = token.Name
		cleanToken.ExpiredTime = token.ExpiredTime
		cleanToken.RemainQuota = token.RemainQuota
		cleanToken.UnlimitedQuota = token.UnlimitedQuota
		cleanToken.ModelLimitsEnabled = token.ModelLimitsEnabled
		cleanToken.ModelLimits = token.ModelLimits
		cleanToken.AllowIps = token.AllowIps
		cleanToken.Group = token.Group
		cleanToken.CrossGroupRetry = token.CrossGroupRetry
		if token.Group != "auto" {
			cleanToken.CrossGroupRetry = false
			_ = cleanToken.SetAutoGroups(nil)
		} else if request.AutoGroups.Set {
			if !setTokenAutoGroups(c, cleanToken, request.AutoGroups.Groups) {
				return
			}
		}
	}
	err = cleanToken.Update()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	action := "token.update"
	if statusOnly != "" {
		action = "token.status_update"
	}
	recordTokenAudit(c, action, cleanToken, nil)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    buildMaskedTokenResponse(cleanToken),
	})
}

type TokenBatch struct {
	Ids []int `json:"ids"`
}

func DeleteTokenBatch(c *gin.Context) {
	tokenBatch := TokenBatch{}
	if err := c.ShouldBindJSON(&tokenBatch); err != nil || len(tokenBatch.Ids) == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userId := c.GetInt("id")
	count, err := model.BatchDeleteTokens(tokenBatch.Ids, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
}

func GetTokenKeysBatch(c *gin.Context) {
	tokenBatch := TokenBatch{}
	if err := c.ShouldBindJSON(&tokenBatch); err != nil || len(tokenBatch.Ids) == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if len(tokenBatch.Ids) > 100 {
		common.ApiErrorI18n(c, i18n.MsgBatchTooMany, map[string]any{"Max": 100})
		return
	}
	userId := c.GetInt("id")
	tokens, err := model.GetTokenKeysByIds(tokenBatch.Ids, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	requestedIDs := make(map[int]struct{}, len(tokenBatch.Ids))
	for _, id := range tokenBatch.Ids {
		if id <= 0 {
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
		requestedIDs[id] = struct{}{}
	}
	if len(tokens) != len(requestedIDs) {
		_ = enqueueCriticalAudit(c, userId, fmt.Sprintf("user:%d:tokens", userId), "token.secret_reveal_batch", "rejected", map[string]interface{}{
			"item_count": len(requestedIDs),
			"reason":     "owner_mismatch_or_missing",
		})
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "API Key 不存在或无权查看", "code": "TOKEN_NOT_FOUND"})
		return
	}
	manager, err := service.CurrentTokenSecretManager()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	keysMap := make(map[int]string)
	for index := range tokens {
		key, revealErr := manager.RevealTokenKey(&tokens[index])
		if revealErr != nil {
			common.ApiError(c, revealErr)
			return
		}
		keysMap[tokens[index].Id] = tokens[index].FormatKeyForClient(key)
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		for index := range tokens {
			token := &tokens[index]
			if err := enqueueCriticalAuditWithTx(c, tx, userId, fmt.Sprintf("token:%d", token.Id), "token.secret_reveal", map[string]interface{}{
				"token_id":   token.Id,
				"name":       token.Name,
				"masked_key": token.GetMaskedKey(),
				"status":     token.Status,
			}); err != nil {
				return err
			}
		}
		return enqueueCriticalAuditWithTx(c, tx, userId, fmt.Sprintf("user:%d:tokens", userId), "token.secret_reveal_batch", map[string]interface{}{
			"item_count": len(tokens),
		})
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, gin.H{"keys": keysMap})
}

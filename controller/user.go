package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/registration_setting"

	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type createUserRequest struct {
	model.User
	AffSkipReason string `json:"aff_skip_reason"`
}

type updateUserRequest struct {
	model.User
	InviteLimitMode  *string `json:"invite_limit_mode"`
	InviteLimitCount *int    `json:"invite_limit_count"`
	Email            *string `json:"email"`
	GitHubId         *string `json:"github_id"`
	DiscordId        *string `json:"discord_id"`
	OidcId           *string `json:"oidc_id"`
	WeChatId         *string `json:"wechat_id"`
	TelegramId       *string `json:"telegram_id"`
	LinuxDOId        *string `json:"linux_do_id"`
}

var (
	errUserPasswordUnset    = errors.New("user password is not set")
	errOriginalPasswordFail = errors.New("original password is incorrect")
)

func validateUserInput(c *gin.Context, user *model.User) bool {
	err := common.Validate.Struct(user)
	if err == nil {
		return true
	}
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, validationError := range validationErrors {
			switch validationError.StructField() {
			case "Username":
				if validationError.Tag() == "max" {
					common.ApiErrorI18n(c, i18n.MsgUserUsernameLengthInvalid)
					return false
				}
			case "Password":
				if validationError.Tag() == "min" || validationError.Tag() == "max" {
					common.ApiErrorI18n(c, i18n.MsgUserPasswordLengthInvalid)
					return false
				}
			}
		}
	}
	common.SysLog(fmt.Sprintf("user input validation failed request_id=%s: %v", c.GetString(common.RequestIdKey), err))
	common.ApiErrorI18n(c, i18n.MsgUserInputInvalid)
	return false
}

func loginAttemptIdentifier(method string, attemptedIdentifier string) string {
	normalized := strings.ToLower(strings.TrimSpace(attemptedIdentifier))
	mac := hmac.New(sha256.New, []byte(common.CryptoSecret))
	_, _ = mac.Write([]byte(strings.TrimSpace(method) + "\x00" + normalized))
	return hex.EncodeToString(mac.Sum(nil))
}

func recordLoginFailureAudit(c *gin.Context, method string, attemptedIdentifier string, category string, targetUserID int) error {
	method = strings.TrimSpace(method)
	if method == "" {
		method = loginMethodFromContext(c)
	}
	identifier := loginAttemptIdentifier(method, attemptedIdentifier)
	target := "login-attempt:" + identifier
	params := map[string]interface{}{
		"attempt_identifier": identifier,
		"failure_category":   category,
		"login_method":       method,
	}
	if targetUserID > 0 {
		target = fmt.Sprintf("user:%d", targetUserID)
		params["target_user_id"] = targetUserID
	}
	return enqueueCriticalAudit(c, 0, target, "user.login_failed", "failure", params)
}

func respondLoginFailureAuditError(c *gin.Context, err error) {
	common.SysLog("failed to enqueue login failure audit: " + err.Error())
	common.ApiErrorI18n(c, i18n.MsgRetryLater)
}

func Login(c *gin.Context) {
	if !common.PasswordLoginEnabled {
		if err := recordLoginFailureAudit(c, "password", "", "method_disabled", 0); err != nil {
			respondLoginFailureAuditError(c, err)
			return
		}
		common.ApiErrorI18n(c, i18n.MsgUserPasswordLoginDisabled)
		return
	}
	var loginRequest LoginRequest
	err := common.DecodeJson(c.Request.Body, &loginRequest)
	if err != nil {
		if auditErr := recordLoginFailureAudit(c, "password", "", "invalid_request", 0); auditErr != nil {
			respondLoginFailureAuditError(c, auditErr)
			return
		}
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	username := loginRequest.Username
	password := loginRequest.Password
	if username == "" || password == "" {
		if auditErr := recordLoginFailureAudit(c, "password", username, "invalid_request", 0); auditErr != nil {
			respondLoginFailureAuditError(c, auditErr)
			return
		}
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user := model.User{
		Username: username,
		Password: password,
	}
	err = user.ValidateAndFill()
	if err != nil {
		failureCategory := "invalid_credentials"
		switch {
		case errors.Is(err, model.ErrDatabase):
			failureCategory = "database_error"
		case errors.Is(err, model.ErrUserEmptyCredentials):
			failureCategory = "invalid_request"
		case errors.Is(err, model.ErrUserDisabled):
			failureCategory = "user_disabled"
		}
		if auditErr := recordLoginFailureAudit(c, "password", username, failureCategory, user.Id); auditErr != nil {
			respondLoginFailureAuditError(c, auditErr)
			return
		}
		switch {
		case errors.Is(err, model.ErrDatabase):
			common.SysLog(fmt.Sprintf("Login database error: %v", err))
			common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		case errors.Is(err, model.ErrUserEmptyCredentials):
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		case errors.Is(err, model.ErrUserDisabled):
			common.ApiErrorMsg(c, user.DisabledMessage())
		default:
			common.ApiErrorI18n(c, i18n.MsgUserUsernameOrPasswordError)
		}
		return
	}

	// 检查是否启用2FA
	if model.IsTwoFAEnabled(user.Id) {
		// 设置pending session，等待2FA验证
		session := sessions.Default(c)
		session.Set("pending_username", user.Username)
		session.Set("pending_user_id", user.Id)
		err := session.Save()
		if err != nil {
			if auditErr := recordLoginFailureAudit(c, "password", user.Username, "session_error", user.Id); auditErr != nil {
				respondLoginFailureAuditError(c, auditErr)
				return
			}
			common.ApiErrorI18n(c, i18n.MsgUserSessionSaveFailed)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": i18n.T(c, i18n.MsgUserRequire2FA),
			"success": true,
			"data": map[string]interface{}{
				"require_2fa": true,
			},
		})
		return
	}

	setupLogin(&user, c)
}

// loginMethodFromContext 根据请求路径推导登录方式，用于登录审计日志。
func loginMethodFromContext(c *gin.Context) string {
	switch c.FullPath() {
	case "/api/user/login":
		return "password"
	case "/api/user/login/2fa":
		return "2fa"
	case "/api/user/passkey/login/finish":
		return "passkey"
	case "/api/oauth/wechat":
		return "wechat"
	case "/api/oauth/telegram/login":
		return "telegram"
	case "/api/oauth/:provider":
		if provider := c.Param("provider"); provider != "" {
			return "oauth:" + provider
		}
		return "oauth"
	default:
		return "unknown"
	}
}

func recordLoginAudit(user *model.User, c *gin.Context) error {
	method := loginMethodFromContext(c)
	return enqueueCriticalAudit(c, user.Id, fmt.Sprintf("user:%d", user.Id), "user.login_success", "success", map[string]interface{}{
		"login_method": method,
	})
}

// setup session & cookies and then return user info
func setupLogin(user *model.User, c *gin.Context) {
	session := sessions.Default(c)
	session.Set("id", user.Id)
	session.Set("username", user.Username)
	session.Set("role", user.Role)
	session.Set("status", user.Status)
	session.Set("group", user.Group)
	session.Set("security_version", user.SecurityVersion)
	err := session.Save()
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserSessionSaveFailed)
		return
	}
	if err := recordLoginAudit(user, c); err != nil {
		session.Clear()
		if clearErr := session.Save(); clearErr != nil {
			common.SysLog("failed to clear session after login audit failure: " + clearErr.Error())
		}
		common.SysLog("failed to enqueue login success audit: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	model.UpdateUserLastLoginAt(user.Id)
	c.JSON(http.StatusOK, gin.H{
		"message": "",
		"success": true,
		"data": map[string]any{
			"id":           user.Id,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"role":         user.Role,
			"status":       user.Status,
			"group":        user.Group,
		},
	})
}

func Logout(c *gin.Context) {
	session := sessions.Default(c)
	userID := c.GetInt("id")
	if sessionUserID, ok := session.Get("id").(int); ok && sessionUserID > 0 {
		userID = sessionUserID
	}
	session.Clear()
	err := session.Save()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	if userID > 0 {
		if err := enqueueCriticalAudit(c, userID, fmt.Sprintf("user:%d", userID), "user.logout", "success", nil); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "",
		"success": true,
	})
}

func Register(c *gin.Context) {
	if !common.RegisterEnabled {
		common.ApiErrorI18n(c, i18n.MsgUserRegisterDisabled)
		return
	}
	if !common.PasswordRegisterEnabled {
		common.ApiErrorI18n(c, i18n.MsgUserPasswordRegisterDisabled)
		return
	}
	var user model.User
	err := common.DecodeJson(c.Request.Body, &user)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user.Username = strings.TrimSpace(user.Username)
	user.Email = model.NormalizeEmail(user.Email)
	if user.Username == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if !validateUserInput(c, &user) {
		return
	}
	if common.EmailVerificationEnabled {
		if user.Email == "" || user.VerificationCode == "" {
			common.ApiErrorI18n(c, i18n.MsgUserEmailVerificationRequired)
			return
		}
		if !common.VerifyCodeWithKey(user.Email, user.VerificationCode, common.EmailVerificationPurpose) {
			common.ApiErrorI18n(c, i18n.MsgUserVerificationCodeError)
			return
		}
		if err := model.EnsureEmailAvailable(user.Email, 0); err != nil {
			if errors.Is(err, model.ErrEmailAlreadyTaken) {
				common.ApiErrorI18n(c, i18n.MsgUserEmailAlreadyTaken)
				return
			}
			common.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
	}
	emailForExistCheck := ""
	if common.EmailVerificationEnabled {
		emailForExistCheck = user.Email
	}
	exist, err := model.CheckUserExistOrDeleted(user.Username, emailForExistCheck)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		common.SysLog(fmt.Sprintf("CheckUserExistOrDeleted error: %v", err))
		return
	}
	if exist {
		common.ApiErrorI18n(c, i18n.MsgUserExists)
		return
	}
	cleanUser := model.User{
		Username:    user.Username,
		Password:    user.Password,
		DisplayName: user.Username,
		Role:        common.RoleCommonUser, // 明确设置角色为普通用户
	}
	if common.EmailVerificationEnabled {
		cleanUser.Email = user.Email
	}
	_, err = service.RegisterWithInvitation(c.Request.Context(), service.RegistrationRequest{
		User: &cleanUser, InvitationCode: user.AffCode, Source: "password", RegistrationIP: c.ClientIP(),
		RequestID: c.GetString(common.RequestIdKey),
	})
	if err != nil {
		if errors.Is(err, model.ErrEmailAlreadyTaken) {
			common.ApiErrorI18n(c, i18n.MsgUserEmailAlreadyTaken)
			return
		}
		common.ApiError(c, err)
		return
	}
	// 获取插入后的用户ID
	if cleanUser.Id == 0 {
		common.ApiErrorI18n(c, i18n.MsgUserRegisterFailed)
		return
	}
	// 生成默认令牌
	if constant.GenerateDefaultToken {
		key, err := common.GenerateAPIKey()
		if err != nil {
			common.ApiErrorI18n(c, i18n.MsgUserDefaultTokenFailed)
			common.SysLog("failed to generate token key: " + err.Error())
			return
		}
		// 生成默认令牌
		token := model.Token{
			UserId:             cleanUser.Id, // 使用插入后的用户ID
			Name:               cleanUser.Username + "的初始令牌",
			Key:                key,
			KeyFormat:          model.TokenKeyFormatFull,
			CreatedTime:        common.GetTimestamp(),
			AccessedTime:       common.GetTimestamp(),
			ExpiredTime:        -1,     // 永不过期
			RemainQuota:        500000, // 示例额度
			UnlimitedQuota:     true,
			ModelLimitsEnabled: false,
		}
		if setting.DefaultUseAutoGroup {
			token.Group = "auto"
		}
		if err := service.InsertProtectedToken(&token); err != nil {
			common.ApiErrorI18n(c, i18n.MsgCreateDefaultTokenErr)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func GetAllUsers(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	sortOptions := model.NewUserSortOptions(c.Query("sort_by"), c.Query("sort_order"))
	users, total, err := model.GetAllUsers(pageInfo, sortOptions)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)

	common.ApiSuccess(c, pageInfo)
	return
}

func SearchUsers(c *gin.Context) {
	keyword := c.Query("keyword")
	group := c.Query("group")
	var role *int
	if roleStr := c.Query("role"); roleStr != "" {
		if parsed, err := strconv.Atoi(roleStr); err == nil {
			role = &parsed
		}
	}
	var status *int
	if statusStr := c.Query("status"); statusStr != "" {
		if parsed, err := strconv.Atoi(statusStr); err == nil {
			status = &parsed
		}
	}
	pageInfo := common.GetPageQuery(c)
	sortOptions := model.NewUserSortOptions(c.Query("sort_by"), c.Query("sort_order"))
	users, total, err := model.SearchUsers(keyword, group, role, status, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), sortOptions)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	common.ApiSuccess(c, pageInfo)
	return
}

func canManageTarget(c *gin.Context, target *model.User) bool {
	return service.CanManageUser(
		service.UserIdentity{ID: c.GetInt("id"), Role: c.GetInt("role")},
		service.UserIdentity{ID: target.Id, Role: target.Role},
	)
}

func GetUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	user, err := model.GetUserById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !canManageTarget(c, user) {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionSameLevel)
		return
	}
	if status, err := model.GetInviteLimitStatus(*user, registration_setting.GetSettings()); err == nil {
		user.InviteLimit = &status
	}
	user.AdminPermissions = authz.Capabilities(user.Id, user.Role)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    user,
	})
	return
}

func GenerateAccessToken(c *gin.Context) {
	id := c.GetInt("id")
	user, err := model.GetUserById(id, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// get rand int 28-32
	randI := common.GetRandomInt(4)
	key, err := common.GenerateRandomKey(29 + randI)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgGenerateFailed)
		common.SysLog("failed to generate key: " + err.Error())
		return
	}
	user.SetAccessToken(key)

	if model.DB.Where("access_token = ?", user.AccessToken).First(user).RowsAffected != 0 {
		common.ApiErrorI18n(c, i18n.MsgUuidDuplicate)
		return
	}

	if err := user.Update(false); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    user.AccessToken,
	})
	return
}

type TransferAffQuotaRequest struct {
	Quota int `json:"quota" binding:"required"`
}

func TransferAffQuota(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	id := c.GetInt("id")
	user, err := model.GetUserById(id, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	tran := TransferAffQuotaRequest{}
	if err := c.ShouldBindJSON(&tran); err != nil {
		common.ApiError(c, err)
		return
	}
	err = user.TransferAffQuotaToQuota(tran.Quota)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserTransferFailed, map[string]any{"Error": err.Error()})
		return
	}
	common.ApiSuccessI18n(c, i18n.MsgUserTransferSuccess, nil)
}

func GetAffCode(c *gin.Context) {
	id := c.GetInt("id")
	user, err := model.GetUserById(id, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user.AffCode == "" {
		user.AffCode = common.GetRandomString(4)
		if err := user.Update(false); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    user.AffCode,
	})
	return
}

func GetSelf(c *gin.Context) {
	id := c.GetInt("id")
	userRole := c.GetInt("role")
	user, err := model.GetUserById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	inviteLimitStatus, err := model.GetInviteLimitStatus(*user, registration_setting.GetSettings())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// Hide admin remarks: set to empty to trigger omitempty tag, ensuring the remark field is not included in JSON returned to regular users
	user.Remark = ""

	// 计算用户权限信息
	permissions := calculateUserPermissions(userRole)
	permissions["admin_permissions"] = authz.Capabilities(id, userRole)

	// 获取用户设置并提取sidebar_modules
	userSetting := user.GetSetting()

	// 构建响应数据，包含用户信息和权限
	responseData := map[string]interface{}{
		"id":                          user.Id,
		"username":                    user.Username,
		"display_name":                user.DisplayName,
		"avatar_url":                  user.AvatarURL,
		"display_name_next_change_at": model.DisplayNameNextChangeAt(*user, time.Now()),
		"role":                        user.Role,
		"status":                      user.Status,
		"email":                       user.Email,
		"github_id":                   user.GitHubId,
		"discord_id":                  user.DiscordId,
		"oidc_id":                     user.OidcId,
		"wechat_id":                   user.WeChatId,
		"telegram_id":                 user.TelegramId,
		"group":                       user.Group,
		"group_name":                  user.Group,
		"quota":                       user.Quota,
		"used_quota":                  user.UsedQuota,
		"request_count":               user.RequestCount,
		"aff_code":                    user.AffCode,
		"aff_count":                   user.AffCount,
		"aff_quota":                   user.AffQuota,
		"aff_history_quota":           user.AffHistoryQuota,
		"inviter_id":                  user.InviterId,
		"inviter_user_id":             user.InviterId,
		"inviter_display_name":        "",
		"invite_limit":                inviteLimitStatus,
		"invite_limit_mode":           user.InviteLimitMode,
		"invite_limit_count":          user.InviteLimitCount,
		"linux_do_id":                 user.LinuxDOId,
		"setting":                     user.Setting,
		"stripe_customer":             user.StripeCustomer,
		"sidebar_modules":             userSetting.SidebarModules, // 正确提取sidebar_modules字段
		"permissions":                 permissions,                // 新增权限字段
		"avatar_upload":               model.GetAvatarUploadPolicy(),
	}
	if inviterInfo := buildInviterInfo(user.InviterId); inviterInfo != nil {
		responseData["inviter_info"] = inviterInfo
		responseData["inviter_display_name"] = inviterInfo["display_name"]
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    responseData,
	})
	return
}

func buildInviterInfo(inviterId int) map[string]interface{} {
	if inviterId == 0 {
		return nil
	}
	inviter, err := model.GetUserById(inviterId, false)
	if err != nil || inviter.Id == 0 {
		return nil
	}
	displayName := strings.TrimSpace(inviter.DisplayName)
	if displayName == "" {
		displayName = inviter.Username
	}
	return map[string]interface{}{
		"username":     inviter.Username,
		"display_name": displayName,
	}
}

// 计算用户权限的辅助函数
func calculateUserPermissions(userRole int) map[string]interface{} {
	permissions := map[string]interface{}{}

	// 根据用户角色计算权限
	if userRole == common.RoleRootUser {
		// 超级管理员不需要边栏设置功能
		permissions["sidebar_settings"] = false
		permissions["sidebar_modules"] = map[string]interface{}{}
	} else if userRole == common.RoleAdminUser {
		// 管理员可以设置边栏，但不包含系统设置功能
		permissions["sidebar_settings"] = true
		permissions["sidebar_modules"] = map[string]interface{}{
			"admin": map[string]interface{}{
				"setting": false, // 管理员不能访问系统设置
			},
		}
	} else {
		// 普通用户只能设置个人功能，不包含管理员区域
		permissions["sidebar_settings"] = true
		permissions["sidebar_modules"] = map[string]interface{}{
			"admin": false, // 普通用户不能访问管理员区域
		}
	}

	return permissions
}

// 根据用户角色生成默认的边栏配置
func generateDefaultSidebarConfig(userRole int) string {
	defaultConfig := map[string]interface{}{}

	// 聊天区域 - 所有用户都可以访问
	defaultConfig["chat"] = map[string]interface{}{
		"enabled":    true,
		"playground": true,
		"chat":       true,
	}

	// 控制台区域 - 所有用户都可以访问
	defaultConfig["console"] = map[string]interface{}{
		"enabled":    true,
		"detail":     true,
		"token":      true,
		"log":        true,
		"midjourney": true,
		"task":       true,
	}

	// 个人中心区域 - 所有用户都可以访问
	defaultConfig["personal"] = map[string]interface{}{
		"enabled":  true,
		"topup":    true,
		"personal": true,
	}

	// 管理员区域 - 根据角色决定
	if userRole == common.RoleAdminUser {
		// 管理员可以访问管理员区域，但不能访问系统设置
		defaultConfig["admin"] = map[string]interface{}{
			"enabled":    true,
			"channel":    true,
			"models":     true,
			"redemption": true,
			"user":       true,
			"setting":    false, // 管理员不能访问系统设置
		}
	} else if userRole == common.RoleRootUser {
		// 超级管理员可以访问所有功能
		defaultConfig["admin"] = map[string]interface{}{
			"enabled":    true,
			"channel":    true,
			"models":     true,
			"redemption": true,
			"user":       true,
			"setting":    true,
		}
	}
	// 普通用户不包含admin区域

	// 转换为JSON字符串
	configBytes, err := common.Marshal(defaultConfig)
	if err != nil {
		common.SysLog("生成默认边栏配置失败: " + err.Error())
		return ""
	}

	return string(configBytes)
}

func GetUserModels(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		id = c.GetInt("id")
	}
	user, err := model.GetUserCache(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	groups := service.GetUserUsableGroups(user.Group)
	group := c.Query("group")
	var groupsToQuery []string
	switch {
	case group == "":
		for usableGroup := range groups {
			groupsToQuery = append(groupsToQuery, usableGroup)
		}
	case group == "auto":
		if _, ok := groups[group]; ok {
			groupsToQuery = service.GetUserAutoGroup(user.Group)
		}
	default:
		if _, ok := groups[group]; ok {
			groupsToQuery = []string{group}
		}
	}

	models := service.GetGroupsEnabledModels(groupsToQuery)
	if c.Query("endpoint_type") == string(constant.EndpointTypeImageGeneration) {
		sort.Strings(models)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    models,
	})
}

func UpdateUser(c *gin.Context) {
	var request updateUserRequest
	err := common.DecodeJson(c.Request.Body, &request)
	updatedUser := request.User
	if err != nil || updatedUser.Id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	updatedUser.Username = strings.TrimSpace(updatedUser.Username)
	if updatedUser.Username == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if updatedUser.Password == "" {
		updatedUser.Password = "$I_LOVE_U" // make Validator happy :)
	}
	originUser, err := model.GetUserById(updatedUser.Id, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if updatedUser.Role != common.RoleGuestUser && updatedUser.Role != originUser.Role {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	updatedUser.Role = originUser.Role
	if request.InviteLimitMode == nil {
		updatedUser.InviteLimitMode = originUser.InviteLimitMode
		updatedUser.InviteLimitCount = originUser.InviteLimitCount
	} else {
		updatedUser.InviteLimitMode = *request.InviteLimitMode
		if request.InviteLimitCount != nil {
			updatedUser.InviteLimitCount = *request.InviteLimitCount
		}
	}
	updatedUser.Email = originUser.Email
	updatedUser.GitHubId = originUser.GitHubId
	updatedUser.DiscordId = originUser.DiscordId
	updatedUser.OidcId = originUser.OidcId
	updatedUser.WeChatId = originUser.WeChatId
	updatedUser.TelegramId = originUser.TelegramId
	updatedUser.LinuxDOId = originUser.LinuxDOId
	if request.Email != nil {
		updatedUser.Email = *request.Email
	}
	if request.GitHubId != nil {
		updatedUser.GitHubId = *request.GitHubId
	}
	if request.DiscordId != nil {
		updatedUser.DiscordId = *request.DiscordId
	}
	if request.OidcId != nil {
		updatedUser.OidcId = *request.OidcId
	}
	if request.WeChatId != nil {
		updatedUser.WeChatId = *request.WeChatId
	}
	if request.TelegramId != nil {
		updatedUser.TelegramId = *request.TelegramId
	}
	if request.LinuxDOId != nil {
		updatedUser.LinuxDOId = *request.LinuxDOId
	}
	if originUser.Username != updatedUser.Username && strings.TrimSpace(updatedUser.DisplayName) == originUser.Username {
		updatedUser.DisplayName = updatedUser.Username
	}
	model.NormalizeEditableUserBindings(&updatedUser)
	originLoginIdentity := *originUser
	model.NormalizeEditableUserBindings(&originLoginIdentity)
	if !canManageTarget(c, originUser) {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionHigherLevel)
		return
	}
	if !validateUserInput(c, &updatedUser) {
		return
	}
	if updatedUser.Password == "$I_LOVE_U" {
		updatedUser.Password = "" // rollback to what it should be
	}
	updatePassword := updatedUser.Password != ""
	securityChanged := updatePassword ||
		updatedUser.Username != originLoginIdentity.Username ||
		updatedUser.Email != originLoginIdentity.Email ||
		updatedUser.GitHubId != originLoginIdentity.GitHubId ||
		updatedUser.DiscordId != originLoginIdentity.DiscordId ||
		updatedUser.OidcId != originLoginIdentity.OidcId ||
		updatedUser.WeChatId != originLoginIdentity.WeChatId ||
		updatedUser.TelegramId != originLoginIdentity.TelegramId ||
		updatedUser.LinuxDOId != originLoginIdentity.LinuxDOId
	authzTouched := false
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := model.EnsureEditableUserBindingsAvailable(tx, &updatedUser, updatedUser.Id); err != nil {
			if errors.Is(err, model.ErrEmailAlreadyTaken) {
				return errors.New(i18n.T(c, i18n.MsgUserEmailAlreadyTaken))
			}
			var bindingErr *model.UserBindingAlreadyTakenError
			if errors.As(err, &bindingErr) {
				return errors.New(i18n.T(c, i18n.MsgUserBindingAlreadyTaken, map[string]any{"Provider": bindingErr.Provider}))
			}
			return err
		}
		password := originUser.Password
		if updatePassword {
			password = updatedUser.Password
		}
		if err := model.EnsureEditableUserHasLoginMethod(tx, updatedUser.Id, password, &updatedUser); err != nil {
			return err
		}
		if err := updatedUser.EditWithTx(tx, updatePassword); err != nil {
			return err
		}
		touched, err := updateAdminPermissionsForUserInTx(c, tx, updatedUser.Id, originUser.Role, updatedUser.AdminPermissions)
		authzTouched = touched
		if err != nil {
			return err
		}
		if securityChanged || authzTouched {
			return model.BumpSecurityVersionWithTx(tx, updatedUser.Id)
		}
		return nil
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	if authzTouched {
		if err := authz.ReloadPolicy(); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	if err := model.InvalidateUserCache(updatedUser.Id); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate user cache for user %d: %s", updatedUser.Id, err.Error()))
	}
	recordManageAuditFor(c, updatedUser.Id, "user.update", map[string]interface{}{
		"username":          updatedUser.Username,
		"username_from":     originUser.Username,
		"username_to":       updatedUser.Username,
		"display_name_from": originUser.DisplayName,
		"display_name_to":   updatedUser.DisplayName,
		"id":                updatedUser.Id,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func AdminClearUserBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	bindingType := strings.ToLower(strings.TrimSpace(c.Param("binding_type")))
	if bindingType == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	user, err := model.GetUserById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if !canManageTarget(c, user) {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionSameLevel)
		return
	}

	if err := model.EnsureCanRemoveLoginBinding(id, bindingType, 0); err != nil {
		common.ApiError(c, err)
		return
	}

	if _, err := user.ClearBinding(bindingType); err != nil {
		common.ApiError(c, err)
		return
	}

	recordManageAuditFor(c, user.Id, "user.binding_clear", map[string]interface{}{
		"bindingType": bindingType,
		"username":    user.Username,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "success",
	})
}

func ClearSelfBinding(c *gin.Context) {
	id := c.GetInt("id")
	bindingType := strings.ToLower(strings.TrimSpace(c.Param("binding_type")))
	if id == 0 || bindingType == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := model.EnsureCanRemoveLoginBinding(id, bindingType, 0); err != nil {
		common.ApiError(c, err)
		return
	}
	user, err := model.GetUserById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	updatedVersion, err := user.ClearBinding(bindingType)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	session := sessions.Default(c)
	session.Set("security_version", updatedVersion)
	if err := session.Save(); err != nil {
		common.ApiError(c, err)
		return
	}
	recordUserSecurityAudit(c, id, "user.binding_clear_self", map[string]interface{}{
		"binding_type": bindingType,
	})
	common.ApiSuccess(c, nil)
}

func UpdateSelf(c *gin.Context) {
	var requestData map[string]interface{}
	if err := common.DecodeJson(c.Request.Body, &requestData); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	// 检查是否是用户设置更新请求 (sidebar_modules 或 language)
	if sidebarModules, sidebarExists := requestData["sidebar_modules"]; sidebarExists {
		userId := c.GetInt("id")
		user, err := model.GetUserById(userId, false)
		if err != nil {
			common.ApiError(c, err)
			return
		}

		// 获取当前用户设置
		currentSetting := user.GetSetting()

		// 更新sidebar_modules字段
		if sidebarModulesStr, ok := sidebarModules.(string); ok {
			currentSetting.SidebarModules = sidebarModulesStr
		}

		if err := model.UpdateUserSetting(user.Id, currentSetting); err != nil {
			common.ApiErrorI18n(c, i18n.MsgUpdateFailed)
			return
		}

		common.ApiSuccessI18n(c, i18n.MsgUpdateSuccess, nil)
		return
	}

	// 检查是否是语言偏好更新请求
	if language, langExists := requestData["language"]; langExists {
		userId := c.GetInt("id")
		user, err := model.GetUserById(userId, false)
		if err != nil {
			common.ApiError(c, err)
			return
		}

		// 获取当前用户设置
		currentSetting := user.GetSetting()

		// 更新language字段
		if langStr, ok := language.(string); ok {
			currentSetting.Language = langStr
		}

		if err := model.UpdateUserSetting(user.Id, currentSetting); err != nil {
			common.ApiErrorI18n(c, i18n.MsgUpdateFailed)
			return
		}

		common.ApiSuccessI18n(c, i18n.MsgUpdateSuccess, nil)
		return
	}

	// 原有的用户信息更新逻辑
	var user model.User
	requestDataBytes, err := common.Marshal(requestData)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err = common.Unmarshal(requestDataBytes, &user); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	if user.Password == "" {
		user.Password = "$I_LOVE_U" // make Validator happy :)
	}
	if err := common.Validate.Struct(&user); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidInput)
		return
	}

	userID := c.GetInt("id")
	currentUser, err := model.GetUserById(userID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user.Password == "$I_LOVE_U" {
		user.Password = ""
	}
	updatePassword, err := checkUpdatePassword(user.OriginalPassword, user.Password, userID)
	if err != nil {
		if errors.Is(err, errUserPasswordUnset) {
			common.ApiErrorI18n(c, i18n.MsgUserPasswordUnset)
			return
		}
		if errors.Is(err, errOriginalPasswordFail) {
			common.ApiErrorI18n(c, i18n.MsgUserOriginalPasswordError)
			return
		}
		common.ApiError(c, err)
		return
	}
	passwordHash := ""
	if updatePassword {
		passwordHash, err = common.Password2Hash(user.Password)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}
	_, hasDisplayName := requestData["display_name"]
	_, hasAvatarURL := requestData["avatar_url"]
	updatedUser, err := model.UpdateOwnProfile(userID, model.OwnProfilePatch{
		DisplayName:    user.DisplayName,
		HasDisplayName: hasDisplayName,
		AvatarURL:      user.AvatarURL,
		HasAvatarURL:   hasAvatarURL,
		PasswordHash:   passwordHash,
	}, time.Now())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if currentUser.DisplayName != updatedUser.DisplayName || currentUser.AvatarURL != updatedUser.AvatarURL {
		recordUserSecurityAudit(c, updatedUser.Id, "user.profile_update", map[string]interface{}{
			"username":          updatedUser.Username,
			"display_name_from": currentUser.DisplayName,
			"display_name_to":   updatedUser.DisplayName,
			"avatar_changed":    currentUser.AvatarURL != updatedUser.AvatarURL,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func checkUpdatePassword(originalPassword string, newPassword string, userId int) (updatePassword bool, err error) {
	if newPassword == "" {
		return
	}
	var currentUser *model.User
	currentUser, err = model.GetUserById(userId, true)
	if err != nil {
		return
	}

	// 密码不为空,需要验证原密码
	if currentUser.Password == "" {
		err = errUserPasswordUnset
		return
	}
	if !common.ValidatePasswordAndHash(originalPassword, currentUser.Password) {
		err = errOriginalPasswordFail
		return
	}
	updatePassword = true
	return
}

func DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	originUser, err := model.GetUserById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	myRole := c.GetInt("role")
	if myRole <= originUser.Role {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionHigherLevel)
		return
	}
	err = model.HardDeleteUserById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, originUser.Id, "user.delete", map[string]interface{}{
		"username": originUser.Username,
		"id":       originUser.Id,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func DeleteSelf(c *gin.Context) {
	id := c.GetInt("id")
	user, _ := model.GetUserById(id, false)

	if user.Role == common.RoleRootUser {
		common.ApiErrorI18n(c, i18n.MsgUserCannotDeleteRootUser)
		return
	}

	err := model.DeleteUserById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func CreateUser(c *gin.Context) {
	var request createUserRequest
	err := common.DecodeJson(c.Request.Body, &request)
	user := request.User
	user.Username = strings.TrimSpace(user.Username)
	if err != nil || user.Username == "" || user.Password == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if !validateUserInput(c, &user) {
		return
	}
	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	exist, err := model.CheckUserExistOrDeleted(user.Username, "")
	if err != nil {
		common.SysLog(fmt.Sprintf("CheckUserExistOrDeleted error: %v", err))
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if exist {
		common.ApiErrorI18n(c, i18n.MsgUserExists)
		return
	}
	myRole := c.GetInt("role")
	if user.Role >= myRole {
		common.ApiErrorI18n(c, i18n.MsgUserCannotCreateHigherLevel)
		return
	}
	affSkipReason := strings.TrimSpace(request.AffSkipReason)
	affCode := strings.TrimSpace(user.AffCode)
	if affCode == "" && registration_setting.GetSettings().AffRequired {
		if affSkipReason == "" {
			common.ApiErrorI18n(c, i18n.MsgUserAffSkipReasonRequired)
			return
		}
	}
	// Even for admin users, we cannot fully trust them!
	cleanUser := model.User{
		Username:    user.Username,
		Password:    user.Password,
		DisplayName: user.DisplayName,
		Role:        user.Role, // 保持管理员设置的角色
	}
	if affCode == "" && affSkipReason != "" {
		cleanUser.AffSkipReason = affSkipReason
		cleanUser.AffSkipOperatorId = c.GetInt("id")
		cleanUser.AffSkipAt = common.GetTimestamp()
	}
	authzTouched := false
	registrationResult, err := service.RegisterWithInvitation(c.Request.Context(), service.RegistrationRequest{
		User: &cleanUser, InvitationCode: affCode, Source: "admin", RegistrationIP: c.ClientIP(),
		ActorID: c.GetInt("id"), RequestID: c.GetString(common.RequestIdKey),
		AllowMissingInvitation: affCode == "" && affSkipReason != "",
		AfterInsert: func(tx *gorm.DB, createdUser *model.User) error {
			touched, err := updateAdminPermissionsForUserInTx(c, tx, createdUser.Id, createdUser.Role, user.AdminPermissions)
			authzTouched = touched
			return err
		},
	})
	if err != nil {
		if model.IsDuplicateUsernameError(err) {
			common.ApiErrorI18n(c, i18n.MsgUserExists)
			return
		}
		common.ApiError(c, err)
		return
	}
	inviterId := registrationResult.InviterID
	if authzTouched {
		if err := authz.ReloadPolicy(); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	auditAction := "user.create"
	if inviterId != 0 {
		auditAction = "user.create.invite"
	} else if affSkipReason != "" {
		auditAction = "user.create.invite_skip"
	}
	recordManageAuditFor(c, cleanUser.Id, auditAction, map[string]interface{}{
		"username":        cleanUser.Username,
		"role":            cleanUser.Role,
		"inviter_id":      inviterId,
		"invite_code_id":  registrationResult.InviteCodeID,
		"aff_skip_reason": affSkipReason,
		"aff_skip_at":     cleanUser.AffSkipAt,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func updateAdminPermissionsForUserInTx(c *gin.Context, tx *gorm.DB, userID int, userRole int, permissions map[string]map[string]bool) (bool, error) {
	if userRole < common.RoleAdminUser && c.GetInt("role") == common.RoleRootUser {
		var overrideCount int64
		if err := tx.Model(&model.CasbinRule{}).
			Where("ptype = ? AND v0 = ?", "p", authz.UserSubject(userID)).
			Count(&overrideCount).Error; err != nil {
			return false, err
		}
		if overrideCount == 0 {
			return false, nil
		}
		return true, authz.ClearUserAuthorizationInTx(tx, userID)
	}
	if permissions == nil {
		return false, nil
	}
	if c.GetInt("role") != common.RoleRootUser {
		return false, fmt.Errorf("only root can update admin permissions")
	}
	if reflect.DeepEqual(authz.PermissionsMap(permissions), authz.Capabilities(userID, userRole)) {
		return false, nil
	}
	return true, authz.SetUserPermissionsInTx(tx, userID, permissions)
}

type ManageRequest struct {
	Id                int    `json:"id"`
	Action            string `json:"action"`
	Value             int64  `json:"value"`
	Mode              string `json:"mode"`
	BanReasonPublic   string `json:"ban_reason_public"`
	BanReasonInternal string `json:"ban_reason_internal"`
}

// ManageUser Only admin user can do this
func ManageUser(c *gin.Context) {
	var req ManageRequest
	err := common.DecodeJson(c.Request.Body, &req)

	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user := model.User{
		Id: req.Id,
	}
	// Fill attributes
	model.DB.Unscoped().Where(&user).First(&user)
	if user.Id == 0 {
		common.ApiErrorI18n(c, i18n.MsgUserNotExists)
		return
	}
	myRole := c.GetInt("role")
	if !canManageTarget(c, &user) {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionHigherLevel)
		return
	}
	originRole := user.Role
	originStatus := user.Status
	switch req.Action {
	case "disable":
		user.Status = common.UserStatusDisabled
		user.BanReasonPublic = strings.TrimSpace(req.BanReasonPublic)
		user.BanReasonInternal = strings.TrimSpace(req.BanReasonInternal)
		if user.Role == common.RoleRootUser {
			common.ApiErrorI18n(c, i18n.MsgUserCannotDisableRootUser)
			return
		}
	case "enable":
		user.Status = common.UserStatusEnabled
		user.BanReasonPublic = ""
		user.BanReasonInternal = ""
	case "delete":
		if user.Role == common.RoleRootUser {
			common.ApiErrorI18n(c, i18n.MsgUserCannotDeleteRootUser)
			return
		}
		if err := user.Delete(); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		// 删除用户后，强制清理 Redis 中所有该用户令牌的缓存，
		// 避免已缓存的令牌在 TTL 过期前仍能通过 TokenAuth 校验。
		if err := model.InvalidateUserTokensCache(user.Id); err != nil {
			common.SysLog(fmt.Sprintf("failed to invalidate tokens cache for user %d: %s", user.Id, err.Error()))
		}
		recordManageAuditFor(c, user.Id, "user.delete", map[string]interface{}{
			"action":   "delete",
			"username": user.Username,
			"id":       user.Id,
		})
		c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
		return
	case "promote":
		if myRole != common.RoleRootUser {
			common.ApiErrorI18n(c, i18n.MsgUserAdminCannotPromote)
			return
		}
		if user.Role >= common.RoleAdminUser {
			common.ApiErrorI18n(c, i18n.MsgUserAlreadyAdmin)
			return
		}
		user.Role = common.RoleAdminUser
	case "promote_root":
		if c.GetInt("id") != 1 || myRole != common.RoleRootUser {
			common.ApiErrorI18n(c, i18n.MsgUserOnlyPrimaryRootCanManageRoot)
			return
		}
		if user.Role == common.RoleRootUser {
			common.ApiErrorI18n(c, i18n.MsgUserAlreadyRoot)
			return
		}
		user.Role = common.RoleRootUser
	case "demote":
		if user.Role == common.RoleRootUser {
			common.ApiErrorI18n(c, i18n.MsgUserCannotDemoteRootUser)
			return
		}
		if user.Role == common.RoleCommonUser {
			common.ApiErrorI18n(c, i18n.MsgUserAlreadyCommon)
			return
		}
		user.Role = common.RoleCommonUser
	case "demote_root":
		if c.GetInt("id") != 1 || myRole != common.RoleRootUser {
			common.ApiErrorI18n(c, i18n.MsgUserOnlyPrimaryRootCanManageRoot)
			return
		}
		if user.Id == 1 {
			common.ApiErrorI18n(c, i18n.MsgUserCannotDemotePrimaryRoot)
			return
		}
		if user.Role != common.RoleRootUser {
			common.ApiErrorI18n(c, i18n.MsgUserNotRoot)
			return
		}
		user.Role = common.RoleAdminUser
	case "add_quota":
		switch req.Mode {
		case "add":
			if req.Value <= 0 {
				common.ApiErrorI18n(c, i18n.MsgUserQuotaChangeZero)
				return
			}
			opID := model.NewAdminQuotaOperationID(c.GetInt("id"), "add", c.GetString(common.RequestIdKey))
			if _, err := service.Adjust(user.Id, req.Value, opID, "admin_adjust", service.WalletReference{
				Type: "admin_quota", ID: fmt.Sprintf("%d", user.Id), Context: "add",
			}); err != nil {
				common.ApiError(c, err)
				return
			}
			recordManageAuditFor(c, user.Id, "user.quota_add", map[string]interface{}{
				"quota":        logger.LogQuota64(req.Value),
				"operation_id": opID,
			})
		case "subtract":
			if req.Value <= 0 {
				common.ApiErrorI18n(c, i18n.MsgUserQuotaChangeZero)
				return
			}
			opID := model.NewAdminQuotaOperationID(c.GetInt("id"), "subtract", c.GetString(common.RequestIdKey))
			if _, err := service.Adjust(user.Id, -req.Value, opID, "admin_adjust", service.WalletReference{
				Type: "admin_quota", ID: fmt.Sprintf("%d", user.Id), Context: "subtract",
			}); err != nil {
				common.ApiError(c, err)
				return
			}
			recordManageAuditFor(c, user.Id, "user.quota_subtract", map[string]interface{}{
				"quota":        logger.LogQuota64(req.Value),
				"operation_id": opID,
			})
		case "override":
			if req.Value < 0 {
				common.ApiError(c, errors.New("quota cannot be negative"))
				return
			}
			var oldQuota int64
			opID := model.NewAdminQuotaOperationID(c.GetInt("id"), "override", c.GetString(common.RequestIdKey))
			err := model.DB.Transaction(func(tx *gorm.DB) error {
				var err error
				oldQuota, err = model.OverrideWalletQuotaWithTx(tx, user.Id, req.Value, opID)
				return err
			})
			if err != nil {
				common.ApiError(c, err)
				return
			}
			_ = model.InvalidateUserCache(user.Id)
			recordManageAuditFor(c, user.Id, "user.quota_override", map[string]interface{}{
				"from":         logger.LogQuota64(oldQuota),
				"to":           logger.LogQuota64(req.Value),
				"operation_id": opID,
			})
		default:
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
		})
		return
	default:
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	securityChanged := req.Action == "disable" || req.Action == "enable" || req.Action == "promote" || req.Action == "demote" || req.Action == "promote_root" || req.Action == "demote_root"
	auditAction := "user.manage"
	auditParams := map[string]interface{}{
		"action":      req.Action,
		"username":    user.Username,
		"id":          user.Id,
		"status_from": originStatus,
		"status_to":   user.Status,
	}
	switch req.Action {
	case "disable":
		auditAction = "user.disable"
		auditParams["ban_reason_public"] = user.BanReasonPublic
		auditParams["ban_reason_internal"] = user.BanReasonInternal
	case "enable":
		auditAction = "user.enable"
	case "promote", "demote", "promote_root", "demote_root":
		auditAction = "user.role_change"
		auditParams["role_from"] = originRole
		auditParams["role_to"] = user.Role
	}
	authzTouched := false
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := user.UpdateWithTx(tx, false); err != nil {
			return err
		}
		if req.Action == "demote" || req.Action == "demote_root" {
			authzTouched = true
			if err := authz.ClearUserAuthorizationInTx(tx, user.Id); err != nil {
				return err
			}
		}
		if securityChanged {
			if err := model.BumpSecurityVersionWithTx(tx, user.Id); err != nil {
				return err
			}
			return enqueueCriticalAuditWithTx(c, tx, c.GetInt("id"), fmt.Sprintf("user:%d", user.Id), auditAction, auditParams)
		}
		return nil
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	if authzTouched {
		if err := authz.ReloadPolicy(); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	markAuditLogged(c)
	// 禁用 / 角色调整后，强制失效用户缓存与其全部令牌缓存，
	// 避免在 Redis TTL 过期前仍使用旧状态（尤其是禁用后仍可发起请求的问题）。
	// InvalidateUserCache 会让下一次 GetUserCache 从数据库重新加载，
	// InvalidateUserTokensCache 则确保令牌侧的缓存也同步刷新。
	if securityChanged {
		if err := model.InvalidateUserCache(user.Id); err != nil {
			common.SysLog(fmt.Sprintf("failed to invalidate user cache for user %d: %s", user.Id, err.Error()))
		}
		if err := model.InvalidateUserTokensCache(user.Id); err != nil {
			common.SysLog(fmt.Sprintf("failed to invalidate tokens cache for user %d: %s", user.Id, err.Error()))
		}
	}
	clearUser := model.User{
		Role:   user.Role,
		Status: user.Status,
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    clearUser,
	})
	return
}

type emailBindRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func EmailBind(c *gin.Context) {
	var req emailBindRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, errors.New("invalid request body"))
		return
	}
	email := req.Email
	email = model.NormalizeEmail(email)
	code := req.Code
	if !common.VerifyCodeWithKey(email, code, common.EmailVerificationPurpose) {
		common.ApiErrorI18n(c, i18n.MsgUserVerificationCodeError)
		return
	}
	id, ok := requireSessionUserId(c)
	if !ok {
		return
	}
	user := model.User{
		Id: id,
	}
	err := user.FillUserById()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.BindEmailToUser(&user, email); err != nil {
		if errors.Is(err, model.ErrEmailAlreadyTaken) {
			common.ApiErrorI18n(c, i18n.MsgUserEmailAlreadyTaken)
			return
		}
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

type topUpRequest struct {
	Key  string `json:"key"`
	Code string `json:"code,omitempty"`
}

var topUpLocks sync.Map
var topUpCreateLock sync.Mutex

type topUpTryLock struct {
	ch chan struct{}
}

func newTopUpTryLock() *topUpTryLock {
	return &topUpTryLock{ch: make(chan struct{}, 1)}
}

func (l *topUpTryLock) TryLock() bool {
	select {
	case l.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

func (l *topUpTryLock) Unlock() {
	select {
	case <-l.ch:
	default:
	}
}

func getTopUpLock(userID int) *topUpTryLock {
	if v, ok := topUpLocks.Load(userID); ok {
		return v.(*topUpTryLock)
	}
	topUpCreateLock.Lock()
	defer topUpCreateLock.Unlock()
	if v, ok := topUpLocks.Load(userID); ok {
		return v.(*topUpTryLock)
	}
	l := newTopUpTryLock()
	topUpLocks.Store(userID, l)
	return l
}

func TopUp(c *gin.Context) {
	if !operation_setting.IsPaymentComplianceConfirmed() {
		common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
		return
	}

	id := c.GetInt("id")
	lock := getTopUpLock(id)
	if !lock.TryLock() {
		common.ApiErrorI18n(c, i18n.MsgUserTopUpProcessing)
		return
	}
	defer lock.Unlock()
	req := topUpRequest{}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	code := req.Key
	if strings.TrimSpace(code) == "" {
		code = req.Code
	}
	result, err := service.RedeemWalletCode(c.Request.Context(), id, code)
	if err != nil {
		apiWalletRedemptionError(c, err)
		logger.LogError(c, fmt.Sprintf("failed to redeem code for user %d: %s", id, err.Error()))
		return
	}
	recordUserSecurityAudit(c, id, "user.redeem_code", map[string]interface{}{
		"quota":     logger.LogQuota64(result.Quota),
		"code_type": result.CodeType,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result.Quota,
	})
}

type UpdateUserSettingRequest struct {
	QuotaWarningType                 string  `json:"notify_type"`
	QuotaWarningThreshold            float64 `json:"quota_warning_threshold"`
	WebhookUrl                       string  `json:"webhook_url,omitempty"`
	WebhookSecret                    string  `json:"webhook_secret,omitempty"`
	NotificationEmail                string  `json:"notification_email,omitempty"`
	BarkUrl                          string  `json:"bark_url,omitempty"`
	GotifyUrl                        string  `json:"gotify_url,omitempty"`
	GotifyToken                      string  `json:"gotify_token,omitempty"`
	GotifyPriority                   int     `json:"gotify_priority,omitempty"`
	UpstreamModelUpdateNotifyEnabled *bool   `json:"upstream_model_update_notify_enabled,omitempty"`
	AcceptUnsetModelRatioModel       bool    `json:"accept_unset_model_ratio_model"`
	RecordIpLog                      bool    `json:"record_ip_log"`
}

func UpdateUserSetting(c *gin.Context) {
	var req UpdateUserSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	// 验证预警类型
	if req.QuotaWarningType != dto.NotifyTypeEmail && req.QuotaWarningType != dto.NotifyTypeWebhook && req.QuotaWarningType != dto.NotifyTypeBark && req.QuotaWarningType != dto.NotifyTypeGotify {
		common.ApiErrorI18n(c, i18n.MsgSettingInvalidType)
		return
	}

	// 验证预警阈值
	if req.QuotaWarningThreshold <= 0 {
		common.ApiErrorI18n(c, i18n.MsgQuotaThresholdGtZero)
		return
	}

	// 如果是webhook类型,验证webhook地址
	if req.QuotaWarningType == dto.NotifyTypeWebhook {
		if req.WebhookUrl == "" {
			common.ApiErrorI18n(c, i18n.MsgSettingWebhookEmpty)
			return
		}
		// 验证URL格式
		if _, err := url.ParseRequestURI(req.WebhookUrl); err != nil {
			common.ApiErrorI18n(c, i18n.MsgSettingWebhookInvalid)
			return
		}
	}

	// 如果是邮件类型，验证邮箱地址
	if req.QuotaWarningType == dto.NotifyTypeEmail && req.NotificationEmail != "" {
		// 验证邮箱格式
		if !strings.Contains(req.NotificationEmail, "@") {
			common.ApiErrorI18n(c, i18n.MsgSettingEmailInvalid)
			return
		}
	}

	// 如果是Bark类型，验证Bark URL
	if req.QuotaWarningType == dto.NotifyTypeBark {
		if req.BarkUrl == "" {
			common.ApiErrorI18n(c, i18n.MsgSettingBarkUrlEmpty)
			return
		}
		// 验证URL格式
		if _, err := url.ParseRequestURI(req.BarkUrl); err != nil {
			common.ApiErrorI18n(c, i18n.MsgSettingBarkUrlInvalid)
			return
		}
		// 检查是否是HTTP或HTTPS
		if !strings.HasPrefix(req.BarkUrl, "https://") && !strings.HasPrefix(req.BarkUrl, "http://") {
			common.ApiErrorI18n(c, i18n.MsgSettingUrlMustHttp)
			return
		}
	}

	// 如果是Gotify类型，验证Gotify URL和Token
	if req.QuotaWarningType == dto.NotifyTypeGotify {
		if req.GotifyUrl == "" {
			common.ApiErrorI18n(c, i18n.MsgSettingGotifyUrlEmpty)
			return
		}
		if req.GotifyToken == "" {
			common.ApiErrorI18n(c, i18n.MsgSettingGotifyTokenEmpty)
			return
		}
		// 验证URL格式
		if _, err := url.ParseRequestURI(req.GotifyUrl); err != nil {
			common.ApiErrorI18n(c, i18n.MsgSettingGotifyUrlInvalid)
			return
		}
		// 检查是否是HTTP或HTTPS
		if !strings.HasPrefix(req.GotifyUrl, "https://") && !strings.HasPrefix(req.GotifyUrl, "http://") {
			common.ApiErrorI18n(c, i18n.MsgSettingUrlMustHttp)
			return
		}
	}

	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	existingSettings := user.GetSetting()
	upstreamModelUpdateNotifyEnabled := existingSettings.UpstreamModelUpdateNotifyEnabled
	if user.Role >= common.RoleAdminUser && req.UpstreamModelUpdateNotifyEnabled != nil {
		upstreamModelUpdateNotifyEnabled = *req.UpstreamModelUpdateNotifyEnabled
	}

	// 构建设置
	settings := dto.UserSetting{
		NotifyType:                       req.QuotaWarningType,
		QuotaWarningThreshold:            req.QuotaWarningThreshold,
		UpstreamModelUpdateNotifyEnabled: upstreamModelUpdateNotifyEnabled,
		AcceptUnsetRatioModel:            req.AcceptUnsetModelRatioModel,
		RecordIpLog:                      true,
	}

	// 如果是webhook类型,添加webhook相关设置
	if req.QuotaWarningType == dto.NotifyTypeWebhook {
		settings.WebhookUrl = req.WebhookUrl
		if req.WebhookSecret != "" {
			settings.WebhookSecret = req.WebhookSecret
		}
	}

	// 如果提供了通知邮箱，添加到设置中
	if req.QuotaWarningType == dto.NotifyTypeEmail && req.NotificationEmail != "" {
		settings.NotificationEmail = req.NotificationEmail
	}

	// 如果是Bark类型，添加Bark URL到设置中
	if req.QuotaWarningType == dto.NotifyTypeBark {
		settings.BarkUrl = req.BarkUrl
	}

	// 如果是Gotify类型，添加Gotify配置到设置中
	if req.QuotaWarningType == dto.NotifyTypeGotify {
		settings.GotifyUrl = req.GotifyUrl
		settings.GotifyToken = req.GotifyToken
		// Gotify优先级范围0-10，超出范围则使用默认值5
		if req.GotifyPriority < 0 || req.GotifyPriority > 10 {
			settings.GotifyPriority = 5
		} else {
			settings.GotifyPriority = req.GotifyPriority
		}
	}

	// 更新用户设置
	if err := model.UpdateUserSetting(user.Id, settings); err != nil {
		common.ApiErrorI18n(c, i18n.MsgUpdateFailed)
		return
	}

	common.ApiSuccessI18n(c, i18n.MsgSettingSaved, nil)
}

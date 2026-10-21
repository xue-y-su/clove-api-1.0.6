package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// providerParams returns map with Provider key for i18n templates
func providerParams(name string) map[string]any {
	return map[string]any{"Provider": name}
}

// GenerateOAuthCode generates a state code for OAuth CSRF protection
func GenerateOAuthCode(c *gin.Context) {
	session := sessions.Default(c)
	state, err := common.GenerateRandomCharsKey(32)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgGenerateFailed)
		common.SysLog(fmt.Sprintf("failed to generate OAuth state: %v", err))
		return
	}
	affCode := c.Query("aff")
	isRegistrationStart := c.Query("register") == "true"
	if isRegistrationStart && session.Get("id") == nil && session.Get("username") == nil {
		if err := service.ValidateInvitationCode(c.Request.Context(), affCode, false); err != nil {
			common.ApiError(c, err)
			return
		}
		if affCode != "" {
			session.Set("aff", affCode)
		} else {
			session.Delete("aff")
		}
	}
	session.Set("oauth_state", state)
	err = session.Save()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    state,
	})
}

// HandleOAuth handles OAuth callback for all standard OAuth providers
func HandleOAuth(c *gin.Context) {
	providerName := c.Param("provider")
	loginMethod := "oauth:" + providerName
	provider := oauth.GetProvider(providerName)
	if provider == nil {
		if err := recordLoginFailureAudit(c, loginMethod, providerName, "provider_unknown", 0); err != nil {
			respondLoginFailureAuditError(c, err)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgOAuthUnknownProvider),
		})
		return
	}

	session := sessions.Default(c)

	// 1. Validate state (CSRF protection)
	state := c.Query("state")
	expectedState, _ := session.Get("oauth_state").(string)
	if state == "" || expectedState == "" || state != expectedState {
		if err := recordLoginFailureAudit(c, loginMethod, providerName, "state_invalid", 0); err != nil {
			respondLoginFailureAuditError(c, err)
			return
		}
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgOAuthStateInvalid),
		})
		return
	}
	session.Delete("oauth_state")
	if err := session.Save(); err != nil {
		if auditErr := recordLoginFailureAudit(c, loginMethod, providerName, "session_error", 0); auditErr != nil {
			respondLoginFailureAuditError(c, auditErr)
			return
		}
		common.ApiError(c, err)
		return
	}

	// 2. Check if user is already logged in (bind flow)
	if session.Get("id") != nil {
		handleOAuthBind(c, provider)
		return
	}
	if session.Get("username") != nil {
		if err := recordLoginFailureAudit(c, loginMethod, providerName, "session_invalid", 0); err != nil {
			respondLoginFailureAuditError(c, err)
			return
		}
		common.ApiErrorI18n(c, i18n.MsgAuthNotLoggedIn)
		return
	}

	// 3. Check if provider is enabled
	if !provider.IsEnabled() {
		if err := recordLoginFailureAudit(c, loginMethod, providerName, "method_disabled", 0); err != nil {
			respondLoginFailureAuditError(c, err)
			return
		}
		common.ApiErrorI18n(c, i18n.MsgOAuthNotEnabled, providerParams(provider.GetName()))
		return
	}

	// 4. Handle error from provider
	errorCode := c.Query("error")
	if errorCode != "" {
		if err := recordLoginFailureAudit(c, loginMethod, providerName, "provider_denied", 0); err != nil {
			respondLoginFailureAuditError(c, err)
			return
		}
		errorDescription := c.Query("error_description")
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": errorDescription,
		})
		return
	}

	// 5. Exchange code for token
	code := c.Query("code")
	token, err := provider.ExchangeToken(c.Request.Context(), code, c)
	if err != nil {
		if auditErr := recordLoginFailureAudit(c, loginMethod, providerName, "token_exchange_failed", 0); auditErr != nil {
			respondLoginFailureAuditError(c, auditErr)
			return
		}
		handleOAuthError(c, err)
		return
	}

	// 6. Get user info
	oauthUser, err := provider.GetUserInfo(c.Request.Context(), token)
	if err != nil {
		if auditErr := recordLoginFailureAudit(c, loginMethod, providerName, "user_info_failed", 0); auditErr != nil {
			respondLoginFailureAuditError(c, auditErr)
			return
		}
		handleOAuthError(c, err)
		return
	}

	// 7. Find or create user
	user, err := findOrCreateOAuthUser(c, provider, oauthUser, session)
	if err != nil {
		failureCategory := "user_resolution_failed"
		if errors.Is(err, model.ErrEmailAlreadyTaken) {
			if auditErr := recordLoginFailureAudit(c, loginMethod, oauthUser.ProviderUserID, "registration_conflict", 0); auditErr != nil {
				respondLoginFailureAuditError(c, auditErr)
				return
			}
			common.ApiErrorI18n(c, i18n.MsgUserEmailAlreadyTaken)
			return
		}
		switch err.(type) {
		case *OAuthUserDeletedError:
			failureCategory = "account_deleted"
		case *OAuthRegistrationDisabledError:
			failureCategory = "registration_disabled"
		case *OAuthEmailAlreadyTakenError:
			failureCategory = "registration_conflict"
		}
		if auditErr := recordLoginFailureAudit(c, loginMethod, oauthUser.ProviderUserID, failureCategory, 0); auditErr != nil {
			respondLoginFailureAuditError(c, auditErr)
			return
		}
		switch err.(type) {
		case *OAuthUserDeletedError:
			common.ApiErrorI18n(c, i18n.MsgOAuthUserDeleted)
		case *OAuthRegistrationDisabledError:
			common.ApiErrorI18n(c, i18n.MsgUserRegisterDisabled)
		case *OAuthEmailAlreadyTakenError:
			common.ApiErrorI18n(c, i18n.MsgUserEmailAlreadyTaken)
		default:
			common.ApiError(c, err)
		}
		return
	}

	// 8. Check user status
	if user.Status != common.UserStatusEnabled {
		if err := recordLoginFailureAudit(c, loginMethod, oauthUser.ProviderUserID, "user_disabled", user.Id); err != nil {
			respondLoginFailureAuditError(c, err)
			return
		}
		common.ApiErrorI18n(c, i18n.MsgOAuthUserBanned)
		return
	}

	// 9. Setup login
	setupLogin(user, c)
}

// handleOAuthBind handles binding OAuth account to existing user
func handleOAuthBind(c *gin.Context, provider oauth.Provider) {
	if !provider.IsEnabled() {
		common.ApiErrorI18n(c, i18n.MsgOAuthNotEnabled, providerParams(provider.GetName()))
		return
	}
	userId, ok := requireSessionUserId(c)
	if !ok {
		return
	}
	user, err := model.GetUserById(userId, false)
	if err != nil || user.Status != common.UserStatusEnabled {
		common.ApiErrorI18n(c, i18n.MsgAuthNotLoggedIn)
		return
	}

	// Exchange code for token
	code := c.Query("code")
	token, err := provider.ExchangeToken(c.Request.Context(), code, c)
	if err != nil {
		handleOAuthError(c, err)
		return
	}

	// Get user info
	oauthUser, err := provider.GetUserInfo(c.Request.Context(), token)
	if err != nil {
		handleOAuthError(c, err)
		return
	}

	// Check if this OAuth account is already bound (check both new ID and legacy ID)
	if provider.IsUserIDTaken(oauthUser.ProviderUserID) {
		common.ApiErrorI18n(c, i18n.MsgOAuthAlreadyBound, providerParams(provider.GetName()))
		return
	}
	// Also check legacy ID to prevent duplicate bindings during migration period
	if legacyID, ok := oauthUser.Extra["legacy_id"].(string); ok && legacyID != "" {
		if provider.IsUserIDTaken(legacyID) {
			common.ApiErrorI18n(c, i18n.MsgOAuthAlreadyBound, providerParams(provider.GetName()))
			return
		}
	}

	// Get current user from session
	// Handle binding based on provider type
	if genericProvider, ok := provider.(*oauth.GenericOAuthProvider); ok {
		// Custom provider: use user_oauth_bindings table
		err = model.UpdateUserOAuthBinding(user.Id, genericProvider.GetProviderId(), oauthUser.ProviderUserID)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	} else {
		// Built-in provider: update user record directly
		provider.SetProviderUserID(user, oauthUser.ProviderUserID)
		err = model.DB.Transaction(func(tx *gorm.DB) error {
			if err := user.UpdateWithTx(tx, false); err != nil {
				return err
			}
			return model.BumpSecurityVersionWithTx(tx, user.Id)
		})
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if err := model.InvalidateUserCache(user.Id); err != nil {
			common.SysLog(fmt.Sprintf("failed to invalidate user cache for user %d after OAuth bind: %s", user.Id, err.Error()))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "bind",
		"data": gin.H{
			"action": "bind",
		},
	})
}

// findOrCreateOAuthUser finds existing user or creates new user
func findOrCreateOAuthUser(c *gin.Context, provider oauth.Provider, oauthUser *oauth.OAuthUser, session sessions.Session) (*model.User, error) {
	user := &model.User{}

	// Check if user already exists with new ID
	if provider.IsUserIDTaken(oauthUser.ProviderUserID) {
		err := provider.FillUserByProviderID(user, oauthUser.ProviderUserID)
		if err != nil {
			return nil, err
		}
		// Check if user has been deleted
		if user.Id == 0 {
			return nil, &OAuthUserDeletedError{}
		}
		return user, nil
	}

	// Try to find user with legacy ID (for GitHub migration from login to numeric ID)
	if legacyID, ok := oauthUser.Extra["legacy_id"].(string); ok && legacyID != "" {
		if provider.IsUserIDTaken(legacyID) {
			err := provider.FillUserByProviderID(user, legacyID)
			if err != nil {
				return nil, err
			}
			if user.Id != 0 {
				// Found user with legacy ID, migrate to new ID
				common.SysLog(fmt.Sprintf("[OAuth] Migrating user %d from legacy_id=%s to new_id=%s",
					user.Id, legacyID, oauthUser.ProviderUserID))
				if err := user.UpdateGitHubId(oauthUser.ProviderUserID); err != nil {
					common.SysError(fmt.Sprintf("[OAuth] Failed to migrate user %d: %s", user.Id, err.Error()))
					// Continue with login even if migration fails
				}
				return user, nil
			}
		}
	}

	// User doesn't exist, create new user if registration is enabled
	if !common.RegisterEnabled {
		return nil, &OAuthRegistrationDisabledError{}
	}

	// Set up new user
	user.Username = provider.GetProviderPrefix() + strconv.Itoa(model.GetMaxUserId()+1)

	if oauthUser.Username != "" {
		if exists, err := model.CheckUserExistOrDeleted(oauthUser.Username, ""); err == nil && !exists {
			// 防止索引退化
			if len(oauthUser.Username) <= model.UserNameMaxLength {
				user.Username = oauthUser.Username
			}
		}
	}

	if oauthUser.DisplayName != "" {
		user.DisplayName = oauthUser.DisplayName
	} else if oauthUser.Username != "" {
		user.DisplayName = oauthUser.Username
	} else {
		user.DisplayName = provider.GetName() + " User"
	}
	if oauthUser.Email != "" {
		user.Email = model.NormalizeEmail(oauthUser.Email)
		if err := model.EnsureEmailAvailable(user.Email, 0); err != nil {
			if errors.Is(err, model.ErrEmailAlreadyTaken) {
				return nil, &OAuthEmailAlreadyTakenError{}
			}
			return nil, err
		}
	}
	user.Role = common.RoleCommonUser
	user.Status = common.UserStatusEnabled

	// Handle affiliate code
	affCode := ""
	if rawAffCode := session.Get("aff"); rawAffCode != nil {
		if value, ok := rawAffCode.(string); ok {
			affCode = value
		}
	}
	var afterInsert func(tx *gorm.DB, createdUser *model.User) error
	if genericProvider, ok := provider.(*oauth.GenericOAuthProvider); ok {
		afterInsert = func(tx *gorm.DB, createdUser *model.User) error {
			binding := &model.UserOAuthBinding{
				UserId:         createdUser.Id,
				ProviderId:     genericProvider.GetProviderId(),
				ProviderUserId: oauthUser.ProviderUserID,
			}
			return model.CreateUserOAuthBindingWithTx(tx, binding)
		}
	} else {
		afterInsert = func(tx *gorm.DB, createdUser *model.User) error {
			provider.SetProviderUserID(createdUser, oauthUser.ProviderUserID)
			return tx.Model(createdUser).Updates(map[string]interface{}{
				"github_id": createdUser.GitHubId, "discord_id": createdUser.DiscordId,
				"oidc_id": createdUser.OidcId, "linux_do_id": createdUser.LinuxDOId,
				"wechat_id": createdUser.WeChatId, "telegram_id": createdUser.TelegramId,
			}).Error
		}
	}
	registrationResult, err := service.RegisterWithInvitation(c.Request.Context(), service.RegistrationRequest{
		User: user, InvitationCode: affCode, Source: "oauth:" + provider.GetName(), RegistrationIP: c.ClientIP(),
		RequestID: c.GetString(common.RequestIdKey),
		AfterInsert: afterInsert,
	})
	if err != nil {
		return nil, err
	}
	user.InviterId = registrationResult.InviterID

	return user, nil
}

// Error types for OAuth
type OAuthUserDeletedError struct{}

func (e *OAuthUserDeletedError) Error() string {
	return "user has been deleted"
}

type OAuthRegistrationDisabledError struct{}

func (e *OAuthRegistrationDisabledError) Error() string {
	return "registration is disabled"
}

type OAuthEmailAlreadyTakenError struct{}

func (e *OAuthEmailAlreadyTakenError) Error() string {
	return "email is already in use"
}

// handleOAuthError handles OAuth errors and returns translated message
func handleOAuthError(c *gin.Context, err error) {
	switch e := err.(type) {
	case *oauth.OAuthError:
		if e.Params != nil {
			common.ApiErrorI18n(c, e.MsgKey, e.Params)
		} else {
			common.ApiErrorI18n(c, e.MsgKey)
		}
	case *oauth.AccessDeniedError:
		common.ApiErrorMsg(c, e.Message)
	case *oauth.TrustLevelError:
		common.ApiErrorI18n(c, i18n.MsgOAuthTrustLevelLow)
	default:
		common.ApiError(c, err)
	}
}

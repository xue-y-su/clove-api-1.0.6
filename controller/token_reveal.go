package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type tokenRevealPasswordRequest struct {
	Password string `json:"password"`
}

func VerifyTokenRevealPassword(c *gin.Context) {
	var request tokenRevealPasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "密码不能为空", "code": "INVALID_REQUEST"})
		return
	}
	user, err := model.GetUserById(c.GetInt("id"), true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user.Password == "" {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "当前账户尚未设置密码，请先通过绑定邮箱设置密码",
			"code":    "PASSWORD_SETUP_REQUIRED",
		})
		return
	}
	if user.Status != common.UserStatusEnabled || !common.ValidatePasswordAndHash(request.Password, user.Password) {
		_ = enqueueCriticalAudit(c, user.Id, fmt.Sprintf("user:%d", user.Id), "token.secret_reveal_verify", "rejected", map[string]interface{}{
			"reason": "invalid_password",
		})
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "密码错误", "code": "INVALID_PASSWORD"})
		return
	}

	now := time.Now().Unix()
	session := sessions.Default(c)
	session.Set(middleware.TokenRevealSessionKey, now)
	if err := session.Save(); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := enqueueCriticalAudit(c, user.Id, fmt.Sprintf("user:%d", user.Id), "token.secret_reveal_verify", "success", nil); err != nil {
		session.Delete(middleware.TokenRevealSessionKey)
		_ = session.Save()
		common.ApiError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"verified_until": now + int64(middleware.TokenRevealWindow/time.Second),
		},
	})
}

type setupOwnPasswordRequest struct {
	Password         string `json:"password"`
	VerificationCode string `json:"verification_code"`
}

var tokenPasswordSetupEmailSender = common.SendEmail

func SendOwnPasswordSetupCode(c *gin.Context) {
	user, err := model.GetUserById(c.GetInt("id"), true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user.Password != "" {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "账户已设置密码", "code": "PASSWORD_ALREADY_SET"})
		return
	}
	email := model.NormalizeEmail(user.Email)
	if email == "" {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "请先绑定邮箱", "code": "EMAIL_BIND_REQUIRED"})
		return
	}
	if !middleware.CheckEmailVerificationRateLimitByEmail(email) {
		c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "message": "验证码发送过于频繁，请稍后再试", "code": "RATE_LIMITED"})
		return
	}
	code := common.GenerateVerificationCode(6)
	common.RegisterVerificationCodeWithKey(email, code, common.EmailVerificationPurpose)
	if err := tokenPasswordSetupEmailSender(fmt.Sprintf("%s邮箱验证邮件", common.SystemName), email, renderEmailVerificationContent(code)); err != nil {
		common.DeleteKey(email, common.EmailVerificationPurpose)
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func SetupOwnPassword(c *gin.Context) {
	var request setupOwnPasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求参数无效", "code": "INVALID_REQUEST"})
		return
	}
	if len(request.Password) < 8 || len(request.Password) > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "密码长度必须为 8 到 20 个字符", "code": "INVALID_PASSWORD"})
		return
	}
	user, err := model.GetUserById(c.GetInt("id"), true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user.Password != "" {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "账户已设置密码", "code": "PASSWORD_ALREADY_SET"})
		return
	}
	email := model.NormalizeEmail(user.Email)
	if email == "" {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "请先绑定邮箱", "code": "EMAIL_BIND_REQUIRED"})
		return
	}
	if !common.VerifyCodeWithKey(email, strings.TrimSpace(request.VerificationCode), common.EmailVerificationPurpose) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "邮箱验证码无效或已过期", "code": "INVALID_VERIFICATION_CODE"})
		return
	}
	passwordHash, err := common.Password2Hash(request.Password)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var updated *model.User
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var updateErr error
		updated, updateErr = model.UpdateOwnProfileWithTx(tx, user.Id, model.OwnProfilePatch{PasswordHash: passwordHash}, time.Now())
		if updateErr != nil {
			return updateErr
		}
		return enqueueCriticalAuditWithTx(c, tx, user.Id, fmt.Sprintf("user:%d", user.Id), "user.password_setup", map[string]interface{}{
			"method": "verified_email",
		})
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.DeleteKey(email, common.EmailVerificationPurpose)
	if err := model.InvalidateUserCache(user.Id); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		common.SysLog(fmt.Sprintf("failed to invalidate user cache after password setup, userId=%d: %s", user.Id, err.Error()))
	}
	// Keep the current authenticated session usable after its own security
	// version changes. Other sessions become stale on their next request.
	session := sessions.Default(c)
	session.Set("security_version", updated.SecurityVersion)
	if err := session.Save(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

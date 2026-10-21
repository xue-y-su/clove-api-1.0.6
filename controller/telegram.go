package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const (
	telegramAuthDateMaxAge     = 10 * time.Minute
	telegramAuthDateFutureSkew = 1 * time.Minute
)

func TelegramBind(c *gin.Context) {
	if !common.TelegramOAuthEnabled {
		c.JSON(200, gin.H{
			"message": "管理员未开启通过 Telegram 登录以及注册",
			"success": false,
		})
		return
	}
	params := c.Request.URL.Query()
	if !checkTelegramAuthorization(params, common.TelegramBotToken) {
		c.JSON(200, gin.H{
			"message": "无效的请求",
			"success": false,
		})
		return
	}
	telegramId := params["id"][0]
	if model.IsTelegramIdAlreadyTaken(telegramId) {
		c.JSON(200, gin.H{
			"message": "该 Telegram 账户已被绑定",
			"success": false,
		})
		return
	}

	userId, ok := requireSessionUserId(c)
	if !ok {
		return
	}
	user := model.User{Id: userId}
	if err := user.FillUserById(); err != nil {
		c.JSON(200, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	if user.Id == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户已注销",
		})
		return
	}
	user.TelegramId = telegramId
	if err := user.Update(false); err != nil {
		c.JSON(200, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}

	c.Redirect(302, common.ThemeAwarePath("/console/personal"))
}

func TelegramLogin(c *gin.Context) {
	if !common.TelegramOAuthEnabled {
		if err := recordLoginFailureAudit(c, "telegram", "", "method_disabled", 0); err != nil {
			respondLoginFailureAuditError(c, err)
			return
		}
		c.JSON(200, gin.H{
			"message": "管理员未开启通过 Telegram 登录以及注册",
			"success": false,
		})
		return
	}
	params := c.Request.URL.Query()
	if !checkTelegramAuthorization(params, common.TelegramBotToken) {
		attemptedID := ""
		if ids := params["id"]; len(ids) > 0 {
			attemptedID = ids[0]
		}
		if err := recordLoginFailureAudit(c, "telegram", attemptedID, "invalid_authorization", 0); err != nil {
			respondLoginFailureAuditError(c, err)
			return
		}
		c.JSON(200, gin.H{
			"message": "无效的请求",
			"success": false,
		})
		return
	}

	telegramId := params["id"][0]
	user := model.User{TelegramId: telegramId}
	if err := user.FillUserByTelegramId(); err != nil {
		if auditErr := recordLoginFailureAudit(c, "telegram", telegramId, "user_lookup_failed", 0); auditErr != nil {
			respondLoginFailureAuditError(c, auditErr)
			return
		}
		c.JSON(200, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	if user.Id == 0 {
		if err := recordLoginFailureAudit(c, "telegram", telegramId, "account_deleted", 0); err != nil {
			respondLoginFailureAuditError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "用户已注销",
			"success": false,
		})
		return
	}
	if user.Status != common.UserStatusEnabled {
		if err := recordLoginFailureAudit(c, "telegram", telegramId, "user_disabled", user.Id); err != nil {
			respondLoginFailureAuditError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "用户已被封禁",
			"success": false,
		})
		return
	}
	setupLogin(&user, c)
}

func checkTelegramAuthorization(params map[string][]string, token string) bool {
	if token == "" {
		return false
	}
	hashValues := params["hash"]
	if len(hashValues) == 0 || hashValues[0] == "" {
		return false
	}
	idValues := params["id"]
	if len(idValues) == 0 || idValues[0] == "" {
		return false
	}
	authDateValues := params["auth_date"]
	if len(authDateValues) == 0 || authDateValues[0] == "" {
		return false
	}
	authDate, err := strconv.ParseInt(authDateValues[0], 10, 64)
	if err != nil {
		return false
	}
	authTime := time.Unix(authDate, 0)
	now := time.Now()
	if authTime.Before(now.Add(-telegramAuthDateMaxAge)) || authTime.After(now.Add(telegramAuthDateFutureSkew)) {
		return false
	}

	strs := []string{}
	hash := hashValues[0]
	for k, v := range params {
		if len(v) == 0 {
			return false
		}
		if k == "hash" {
			continue
		}
		strs = append(strs, k+"="+v[0])
	}
	sort.Strings(strs)
	var imploded = ""
	for _, s := range strs {
		if imploded != "" {
			imploded += "\n"
		}
		imploded += s
	}
	sha256hash := sha256.New()
	io.WriteString(sha256hash, token)
	hmachash := hmac.New(sha256.New, sha256hash.Sum(nil))
	io.WriteString(hmachash, imploded)
	providedHash, err := hex.DecodeString(hash)
	if err != nil {
		return false
	}
	return hmac.Equal(providedHash, hmachash.Sum(nil))
}

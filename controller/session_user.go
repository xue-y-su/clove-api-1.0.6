package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func requireSessionUserId(c *gin.Context) (int, bool) {
	rawId := sessions.Default(c).Get("id")
	userId, ok := rawId.(int)
	if !ok || userId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgAuthNotLoggedIn)
		return 0, false
	}
	return userId, true
}

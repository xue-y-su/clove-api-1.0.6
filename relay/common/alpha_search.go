package common

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func GenRelayInfoAlphaSearch(c *gin.Context, request *dto.AlphaSearchRequest) *RelayInfo {
	info := genBaseRelayInfo(c, request)
	info.RelayFormat = types.RelayFormatOpenAIAlphaSearch
	return info
}

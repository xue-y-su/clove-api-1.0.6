package relay

import (
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func finalizeRelayResponseBilling(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage, extraContent []string) *types.NewAPIError {
	err := service.FinalizeRelayBilling(c, info, usage, relayDeliveryState(c, info, extraContent))
	return service.RelayBillingFinalizationError(err)
}

func finalizeAudioResponseBilling(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage, extraContent string) *types.NewAPIError {
	err := service.FinalizeAudioBilling(c, info, usage, extraContent, relayDeliveryState(c, info, nil))
	return service.RelayBillingFinalizationError(err)
}

func relayDeliveryState(c *gin.Context, info *relaycommon.RelayInfo, extraContent []string) service.DeliveryState {
	forwarded := false
	if c != nil && c.Writer != nil {
		if state, ok := c.Writer.(interface{ ResponseForwarded() bool }); ok {
			forwarded = state.ResponseForwarded()
		} else {
			forwarded = c.Writer.Written()
		}
	}
	return service.DeliveryState{
		Stream:            info != nil && info.IsStream,
		ResponseForwarded: forwarded,
		ExtraContent:      extraContent,
	}
}

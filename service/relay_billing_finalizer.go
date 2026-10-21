package service

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type DeliveryState struct {
	Stream            bool
	ResponseForwarded bool
	ExtraContent      []string
}

func RelayBillingFinalizationError(err error) *types.NewAPIError {
	if err == nil {
		return nil
	}
	if errors.Is(err, model.ErrWalletInsufficientBalance) ||
		errors.Is(err, model.ErrNoActiveSubscription) ||
		errors.Is(err, model.ErrSubscriptionQuotaInsufficient) {
		return types.NewErrorWithStatusCode(
			err,
			types.ErrorCodeInsufficientUserQuota,
			http.StatusForbidden,
			types.ErrOptionWithSkipRetry(),
			types.ErrOptionWithNoRecordErrorLog(),
		)
	}
	return types.NewErrorWithStatusCode(
		err,
		types.ErrorCodeUpdateDataError,
		http.StatusInternalServerError,
		types.ErrOptionWithSkipRetry(),
	)
}

func FinalizeRelayBilling(ctx *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage, delivery DeliveryState) error {
	return finalizeBillingForDelivery(ctx, info, delivery, func() error {
		return finalizeRelayBilling(ctx, info, usage, delivery.ExtraContent)
	})
}

func FinalizeAudioBilling(ctx *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage, extraContent string, delivery DeliveryState) error {
	return finalizeBillingForDelivery(ctx, info, delivery, func() error {
		return finalizeAudioBilling(ctx, info, usage, extraContent)
	})
}

func FinalizeWssBilling(ctx *gin.Context, info *relaycommon.RelayInfo, modelName string, usage *dto.RealtimeUsage, extraContent string, delivery DeliveryState) error {
	return finalizeBillingForDelivery(ctx, info, delivery, func() error {
		return finalizeWssBilling(ctx, info, modelName, usage, extraContent)
	})
}

func finalizeBillingForDelivery(ctx *gin.Context, info *relaycommon.RelayInfo, delivery DeliveryState, finalize func() error) error {
	err := info.FinalizeBillingOnce(finalize)
	if err != nil && delivery.ResponseForwarded && info != nil && info.Billing != nil {
		if preserver, ok := info.Billing.(interface {
			PreserveReservationAfterDeliveryWithError() error
		}); ok {
			if preserveErr := preserver.PreserveReservationAfterDeliveryWithError(); preserveErr != nil {
				logger.LogError(ctx, fmt.Sprintf(
					"CRITICAL billing reservation preservation failed request_id=%s user_id=%d error=%v",
					info.RequestId,
					info.UserId,
					preserveErr,
				))
			}
		} else if preserver, ok := info.Billing.(interface{ PreserveReservationAfterDelivery() }); ok {
			preserver.PreserveReservationAfterDelivery()
		}
	}
	return err
}

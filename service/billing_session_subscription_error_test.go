package service

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type subscriptionErrorFunding struct {
	err error
}

func (*subscriptionErrorFunding) Source() string         { return BillingSourceSubscription }
func (f *subscriptionErrorFunding) PreConsume(int) error { return f.err }
func (*subscriptionErrorFunding) Settle(int) error       { return nil }
func (*subscriptionErrorFunding) Refund() error          { return nil }

func TestBillingSessionMapsSubscriptionSentinelErrorsToInsufficientQuota(t *testing.T) {
	tests := map[string]error{
		"no active subscription": model.ErrNoActiveSubscription,
		"quota insufficient":     fmt.Errorf("%w: need=100", model.ErrSubscriptionQuotaInsufficient),
	}

	for name, fundingError := range tests {
		t.Run(name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			session := &BillingSession{
				relayInfo: &relaycommon.RelayInfo{},
				funding:   &subscriptionErrorFunding{err: fundingError},
			}

			apiErr := session.preConsume(context, 0)

			require.NotNil(t, apiErr)
			assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
		})
	}
}

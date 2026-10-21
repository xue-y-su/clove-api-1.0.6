package controller

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestTopUpPayerInfoIsAdminOnly(t *testing.T) {
	topUp := &model.TopUp{
		Id: 1, UserId: 7, TradeNo: "payer-visibility", PayerEmail: "payer@example.com", PayerName: "Payment Customer",
	}

	userPayload, err := json.Marshal(topUp)
	require.NoError(t, err)
	require.NotContains(t, string(userPayload), "payer@example.com")
	require.NotContains(t, string(userPayload), "Payment Customer")

	adminPayload, err := json.Marshal(newAdminTopUpRecord(topUp))
	require.NoError(t, err)
	require.Contains(t, string(adminPayload), `"payer_email":"payer@example.com"`)
	require.Contains(t, string(adminPayload), `"payer_name":"Payment Customer"`)
}

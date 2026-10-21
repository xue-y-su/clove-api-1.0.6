package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func signedTelegramParams(t *testing.T, token string, values url.Values) map[string][]string {
	t.Helper()
	pairs := make([]string, 0, len(values))
	for key, value := range values {
		if key == "hash" || len(value) == 0 {
			continue
		}
		pairs = append(pairs, key+"="+value[0])
	}
	sort.Strings(pairs)
	secret := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, secret[:])
	_, err := mac.Write([]byte(strings.Join(pairs, "\n")))
	require.NoError(t, err)
	values.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return map[string][]string(values)
}

func TestCheckTelegramAuthorizationRejectsExpiredAuthDate(t *testing.T) {
	token := "telegram-token"
	params := signedTelegramParams(t, token, url.Values{
		"id":        {"12345"},
		"auth_date": {strconv.FormatInt(time.Now().Add(-20*time.Minute).Unix(), 10)},
	})

	require.False(t, checkTelegramAuthorization(params, token))
}

func TestCheckTelegramAuthorizationRequiresId(t *testing.T) {
	token := "telegram-token"
	params := signedTelegramParams(t, token, url.Values{
		"auth_date": {strconv.FormatInt(time.Now().Unix(), 10)},
	})

	require.False(t, checkTelegramAuthorization(params, token))
}

func TestCheckTelegramAuthorizationAllowsRecentAuthDate(t *testing.T) {
	token := "telegram-token"
	params := signedTelegramParams(t, token, url.Values{
		"id":        {"12345"},
		"auth_date": {strconv.FormatInt(time.Now().Unix(), 10)},
	})

	require.True(t, checkTelegramAuthorization(params, token))
}

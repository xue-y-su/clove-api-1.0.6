package common

import "github.com/QuantumNous/new-api/constant"

const defaultAnonymousRequestBodyLimitKB = 512
const defaultAPIRequestBodyLimitKB = 8192

func GetAnonymousRequestBodyLimitBytes() int64 {
	limitKB := constant.AnonymousRequestBodyLimitKB
	if limitKB < 0 {
		limitKB = defaultAnonymousRequestBodyLimitKB
	}
	return int64(limitKB) << 10
}

func GetAPIRequestBodyLimitBytes() int64 {
	limitKB := constant.APIRequestBodyLimitKB
	if limitKB < 0 {
		limitKB = defaultAPIRequestBodyLimitKB
	}
	return int64(limitKB) << 10
}

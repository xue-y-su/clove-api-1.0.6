package common

import (
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/constant"
)

const defaultUpstreamResponseBodyLimitMB = 128

func GetUpstreamResponseBodyLimitBytes() int64 {
	limitMB := constant.UpstreamResponseBodyLimitMB
	if limitMB < 0 {
		limitMB = defaultUpstreamResponseBodyLimitMB
	}
	return int64(limitMB) << 20
}

func LimitUpstreamResponseBody(body io.ReadCloser) io.ReadCloser {
	if body == nil {
		return nil
	}
	maxBytes := GetUpstreamResponseBodyLimitBytes()
	if maxBytes <= 0 {
		return body
	}
	return http.MaxBytesReader(nil, body, maxBytes)
}

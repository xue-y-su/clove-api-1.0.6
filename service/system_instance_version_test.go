package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestCurrentSystemInstanceRuntimeUsesDisplayVersion(t *testing.T) {
	previousVersion := common.Version
	common.Version = "v1.0.0-rc.21.clove-enhanced"
	t.Cleanup(func() { common.Version = previousVersion })

	runtimeInfo := currentSystemInstanceRuntimeInfo()

	require.Equal(t, common.DisplayVersion, runtimeInfo.Version)
	require.NotEqual(t, common.Version, runtimeInfo.Version)
	require.NotEmpty(t, runtimeInfo.GOOS)
	require.NotEmpty(t, runtimeInfo.GOARCH)
	require.Equal(t, common.StartTime, runtimeInfo.StartedAt)
}

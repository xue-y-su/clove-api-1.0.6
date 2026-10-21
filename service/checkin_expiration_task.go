package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	checkinExpirationTickInterval = time.Minute
	checkinExpirationBatchSize    = 300
)

var (
	checkinExpirationOnce    sync.Once
	checkinExpirationRunning atomic.Bool
	settleDueCheckinRewards  = model.SettleDueCheckinRewards
)

func checkinExpirationTaskEnabled() bool {
	return common.IsMasterNode
}

func StartCheckinExpirationTask() {
	if !checkinExpirationTaskEnabled() {
		return
	}
	checkinExpirationOnce.Do(func() {
		gopool.Go(func() {
			ticker := time.NewTicker(checkinExpirationTickInterval)
			defer ticker.Stop()

			runCheckinExpirationOnce()
			for range ticker.C {
				runCheckinExpirationOnce()
			}
		})
	})
}

func runCheckinExpirationOnce() {
	if !checkinExpirationRunning.CompareAndSwap(false, true) {
		return
	}
	defer checkinExpirationRunning.Store(false)

	for {
		settled, err := settleDueCheckinRewards(checkinExpirationBatchSize)
		if err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("check-in reward expiration task failed: %v", err))
			return
		}
		if settled < checkinExpirationBatchSize {
			return
		}
	}
}

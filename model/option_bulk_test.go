package model

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionsBulkPublishesOptionMapAtomically(t *testing.T) {
	db := openSourceAttributionModelTestDB(t)
	require.NoError(t, db.AutoMigrate(&Option{}))
	preserveOptionTestGlobals(t)

	const optionCount = 512
	values := make(map[string]string, optionCount)
	common.OptionMapRWMutex.Lock()
	for index := 0; index < optionCount; index++ {
		key := fmt.Sprintf("bulk.atomic.%04d", index)
		common.OptionMap[key] = "old"
		values[key] = "new"
	}
	common.OptionMapRWMutex.Unlock()

	var mixed atomic.Bool
	var readers sync.WaitGroup
	stop := make(chan struct{})
	for range 8 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				updated := 0
				common.OptionMapRWMutex.RLock()
				for key := range values {
					if common.OptionMap[key] == "new" {
						updated++
					}
				}
				common.OptionMapRWMutex.RUnlock()
				if updated > 0 && updated < optionCount {
					mixed.Store(true)
					return
				}
				runtime.Gosched()
			}
		}()
	}

	require.NoError(t, UpdateOptionsBulk(values))
	close(stop)
	readers.Wait()
	require.False(t, mixed.Load(), "readers observed a partially published option set")
}

func TestUpdateOptionDoesNotMutateMemoryAfterDatabaseFailure(t *testing.T) {
	db := openSourceAttributionModelTestDB(t)
	require.NoError(t, db.AutoMigrate(&Option{}))
	preserveOptionTestGlobals(t)
	common.OptionMapRWMutex.Lock()
	common.OptionMap["SMTPServer"] = "old.example.test"
	common.OptionMapRWMutex.Unlock()

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	require.Error(t, UpdateOption("SMTPServer", "new.example.test"))
	common.OptionMapRWMutex.RLock()
	stored := common.OptionMap["SMTPServer"]
	common.OptionMapRWMutex.RUnlock()
	require.Equal(t, "old.example.test", stored)
}

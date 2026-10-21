package common

import "errors"

func (info *RelayInfo) FinalizeBillingOnce(finalize func() error) error {
	if info == nil {
		return errors.New("relay info is nil")
	}
	if finalize == nil {
		return errors.New("billing finalizer is nil")
	}

	info.billingFinalizeMu.Lock()
	defer info.billingFinalizeMu.Unlock()
	if info.billingFinalized {
		return info.billingFinalizeErr
	}
	info.billingFinalizeErr = finalize()
	info.billingFinalized = true
	return info.billingFinalizeErr
}

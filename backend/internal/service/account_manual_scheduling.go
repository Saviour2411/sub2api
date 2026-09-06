package service

import "context"

const AccountManualSchedulingPauseKey = "manual_scheduling_paused"

func (a *Account) IsManuallySchedulingPaused() bool {
	if a == nil {
		return false
	}
	paused, _ := a.Extra[AccountManualSchedulingPauseKey].(bool)
	return paused
}

type adminAccountSchedulingRepository interface {
	SetAdminSchedulable(context.Context, int64, bool) error
}

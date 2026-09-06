//go:build unit

package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFirstToken超时沿用换号上限且不在同账号重试(t *testing.T) {
	state := NewFailoverState(5, false)
	recorder := &mockTempUnscheduler{}
	for id := int64(1); id <= 6; id++ {
		action := state.HandleFailoverError(context.Background(), recorder, id, service.PlatformOpenAI, firstTokenTimeoutHandlerError(), 10)
		if id <= 5 {
			require.Equal(t, FailoverContinue, action, "前三个账号失败不能提前耗尽")
		} else {
			require.Equal(t, FailoverExhausted, action)
		}
		require.Contains(t, state.FailedAccountIDs, id)
	}
	require.Empty(t, state.SameAccountRetryCount)
	require.Equal(t, 5, state.SwitchCount)
}

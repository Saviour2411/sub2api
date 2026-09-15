package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRegistryNeverDeclaresUnknownPeerDead(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = client.Close() }()
	r := Registry{Redis: client}
	ctx := context.Background()
	dead, err := r.Dead(ctx, "legacy")
	require.NoError(t, err)
	require.False(t, dead)
	require.NoError(t, r.Publish(ctx, Instance{ID: "a", Socket: "/run/a.sock"}))
	dead, err = r.Dead(ctx, "a")
	require.NoError(t, err)
	require.False(t, dead)
	server.FastForward(61 * time.Second)
	dead, err = r.Dead(ctx, "a")
	require.NoError(t, err)
	require.True(t, dead)
	server.Close()
	_, err = r.Dead(ctx, "a")
	require.Error(t, err)
}

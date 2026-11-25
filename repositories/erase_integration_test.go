//go:build integration

package repositories

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Everything of the deleted user goes, in one transaction, and nothing of anyone else.
func TestEraseUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	repo := NewEraseRepository(store)
	const gone, kept = "00000000-0000-4000-8000-0000000000e1", "00000000-0000-4000-8000-0000000000e2"
	const count = `SELECT count(*) FROM subscriptions WHERE user_id = $1 AND stopped_at IS NULL`
	left := func(id string) int {
		var n int
		require.NoError(t, store.Conn(ctx).QueryRow(ctx, count, id).Scan(&n))
		return n
	}
	for _, id := range []string{gone, kept} {
		for _, q := range []string{
			`INSERT INTO subscriptions (user_id, plan_id, status, starts_at, ends_at) SELECT $1, id, 'active', now(), now() + interval '30 days' FROM plans WHERE name = 'Free'`,
		} {
			_, err := store.Conn(ctx).Exec(ctx, q, id)
			require.NoError(t, err, q)
		}
	}
	require.Equal(t, 1, left(gone))

	rows, err := repo.EraseUser(ctx, gone)
	require.NoError(t, err)
	assert.Positive(t, rows)
	assert.Zero(t, left(gone))
	assert.Equal(t, 1, left(kept), "another student keeps everything")

	// The subscription and what was paid stay as records: only the renewal is stopped.
	var n int
	require.NoError(t, store.Conn(ctx).QueryRow(ctx, `SELECT count(*) FROM subscriptions WHERE user_id = $1`, gone).Scan(&n))
	assert.Equal(t, 1, n)

	rows, err = repo.EraseUser(ctx, gone)
	require.NoError(t, err, "erasing again is not an error")
	assert.Zero(t, rows)
}

//go:build integration

package repositories

import (
	"context"
	"testing"

	"9itchi-payments-api/migrations"
	"9itchi-payments-api/models/db"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrationsCreateTables(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	for _, table := range []string{"plans", "promo_codes", "subscriptions", "payments"} {
		var n int
		require.NoError(t, store.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n), table)
		if table == "plans" {
			assert.Equal(t, 1, n, "Free plan is seeded")
		} else {
			assert.Equal(t, 0, n)
		}
	}
}

func TestMigrationRecruiterSearches(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	sqlDB := stdlib.OpenDB(*store.pool.Config().ConnConfig)
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)

	_, err = provider.DownTo(ctx, 17)
	require.NoError(t, err)
	repo := NewPlanRepository(store)
	_, err = repo.Create(ctx, &db.Plan{Name: "Pro", Price: 9.99, DurationDays: 30, Quotas: map[string]int{"resumes": 10}})
	require.NoError(t, err)
	_, err = provider.UpTo(ctx, 18)
	require.NoError(t, err)

	pro, err := repo.GetByName(ctx, "Pro")
	require.NoError(t, err)
	assert.Equal(t, 6, pro.Quotas["recruiter_searches_per_day"], "every paid plan, even switched off")
	assert.Equal(t, 10, pro.Quotas["resumes"])
	free, err := repo.GetByName(ctx, "Free")
	require.NoError(t, err)
	assert.Equal(t, 0, free.Quotas["recruiter_searches_per_day"])
}

func TestUUIDScansIntoString(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	var id string
	require.NoError(t, store.Conn(ctx).QueryRow(ctx, "SELECT gen_random_uuid()").Scan(&id))
	assert.Len(t, id, 36)
}

func TestWithTxRollsBack(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	_ = store.WithTx(ctx, func(ctx context.Context) error {
		_, err := store.Conn(ctx).Exec(ctx, "CREATE TEMP TABLE x(a int)")
		require.NoError(t, err)
		return assert.AnError
	})
	var n int
	err := store.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM x").Scan(&n)
	assert.Error(t, err)
}

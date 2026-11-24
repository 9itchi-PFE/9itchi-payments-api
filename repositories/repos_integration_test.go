//go:build integration

package repositories

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"9itchi-payments-api/models/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPromo(code string, usageLimit int, planIDs []string) *db.PromoCode {
	if planIDs == nil {
		planIDs = []string{}
	}
	now := time.Now()
	return &db.PromoCode{Code: code, Type: "percentage", Value: 10, ValidFrom: now.Add(-time.Hour),
		ValidUntil: now.Add(time.Hour), UsageLimit: usageLimit, PlanIDs: planIDs}
}

func TestPlanRepository(t *testing.T) {
	store := newTestStore(t)
	repo := NewPlanRepository(store)
	ctx := context.Background()

	free, err := repo.GetByName(ctx, "Free")
	require.NoError(t, err)
	assert.Equal(t, 0.0, free.Price)
	assert.Equal(t, 36500, free.DurationDays)
	assert.Equal(t, map[string]int{"emails_per_month": 0, "resumes": 1, "cv_downloads": 1}, free.Quotas)

	pro, err := repo.Create(ctx, &db.Plan{Name: "Pro", Price: 9.99, DurationDays: 30, Quotas: map[string]int{"resumes": 10}, Active: true})
	require.NoError(t, err)
	assert.Len(t, pro.ID, 36)
	assert.InDelta(t, 9.99, pro.Price, 0.001)

	_, err = repo.Create(ctx, &db.Plan{Name: "Pro", Price: 1, DurationDays: 1, Quotas: map[string]int{}, Active: true})
	assert.ErrorIs(t, err, ErrConflict)

	_, err = repo.Create(ctx, &db.Plan{Name: "Bad", Price: -1, DurationDays: 1, Quotas: map[string]int{}})
	assert.Error(t, err, "CHECK price >= 0")
	_, err = repo.Create(ctx, &db.Plan{Name: "Bad", Price: 1, DurationDays: 0, Quotas: map[string]int{}})
	assert.Error(t, err, "CHECK duration_days > 0")

	updated, err := repo.Update(ctx, pro.ID, &db.Plan{Name: "Pro", Price: 12, DurationDays: 60, Quotas: map[string]int{"resumes": 20}, Active: true})
	require.NoError(t, err)
	assert.Equal(t, 60, updated.DurationDays)
	assert.Equal(t, 20, updated.Quotas["resumes"])
	assert.False(t, updated.UpdatedAt.Before(pro.UpdatedAt))

	_, err = repo.Update(ctx, "00000000-0000-0000-0000-000000000000", &db.Plan{Name: "X", DurationDays: 1, Quotas: map[string]int{}})
	assert.ErrorIs(t, err, ErrNotFound)

	assert.Nil(t, updated.RegularPrice)
	regular := 50.0
	withRegular, err := repo.Update(ctx, pro.ID, &db.Plan{Name: "Pro", Price: 35, RegularPrice: &regular, DurationDays: 30, Quotas: map[string]int{}, Active: true})
	require.NoError(t, err)
	require.NotNil(t, withRegular.RegularPrice)
	assert.InDelta(t, 50, *withRegular.RegularPrice, 0.001)
	low := 35.0
	_, err = repo.Update(ctx, pro.ID, &db.Plan{Name: "Pro", Price: 35, RegularPrice: &low, DurationDays: 30, Quotas: map[string]int{}, Active: true})
	assert.Error(t, err, "CHECK regular_price > price")

	off, err := repo.Deactivate(ctx, pro.ID)
	require.NoError(t, err)
	assert.False(t, off.Active)

	active, err := repo.List(ctx, true)
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, "Free", active[0].Name)

	all, err := repo.List(ctx, false)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	_, err = repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPromoRepositoryCRUD(t *testing.T) {
	store := newTestStore(t)
	plans := NewPlanRepository(store)
	repo := NewPromoRepository(store)
	ctx := context.Background()

	free, err := plans.GetByName(ctx, "Free")
	require.NoError(t, err)

	p, err := repo.Create(ctx, newPromo("WELCOME", 0, []string{free.ID}))
	require.NoError(t, err)
	assert.Equal(t, []string{free.ID}, p.PlanIDs)
	assert.Equal(t, 0, p.UsedCount)

	all, err := repo.Create(ctx, newPromo("ALLPLANS", 5, nil))
	require.NoError(t, err)
	assert.Empty(t, all.PlanIDs)

	_, err = repo.Create(ctx, newPromo("WELCOME", 0, nil))
	assert.ErrorIs(t, err, ErrConflict)

	lower := newPromo("lower", 0, nil)
	_, err = repo.Create(ctx, lower)
	assert.Error(t, err, "CHECK code stored upper-case")

	over := newPromo("OVER", 0, nil)
	over.Value = 101
	_, err = repo.Create(ctx, over)
	assert.Error(t, err, "CHECK percentage <= 100")

	fixedBig := newPromo("FIXEDBIG", 0, nil)
	fixedBig.Type, fixedBig.Value = "fixed", 500
	_, err = repo.Create(ctx, fixedBig)
	assert.NoError(t, err, "fixed amount may exceed 100")

	badDates := newPromo("DATES", 0, nil)
	badDates.ValidFrom, badDates.ValidUntil = badDates.ValidUntil, badDates.ValidFrom
	_, err = repo.Create(ctx, badDates)
	assert.Error(t, err, "CHECK valid_from < valid_until")

	badType := newPromo("TYPE", 0, nil)
	badType.Type = "weird"
	_, err = repo.Create(ctx, badType)
	assert.Error(t, err, "CHECK type")

	got, err := repo.GetByCode(ctx, "WELCOME")
	require.NoError(t, err)
	assert.Equal(t, p.ID, got.ID)
	_, err = repo.GetByCode(ctx, "NOPE")
	assert.ErrorIs(t, err, ErrNotFound)

	upd := newPromo("WELCOME2", 3, nil)
	upd.Value = 25
	out, err := repo.Update(ctx, p.ID, upd)
	require.NoError(t, err)
	assert.Equal(t, "WELCOME2", out.Code)
	assert.Equal(t, 3, out.UsageLimit)
	assert.Empty(t, out.PlanIDs)

	list, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 3)

	require.NoError(t, repo.Delete(ctx, p.ID))
	assert.ErrorIs(t, repo.Delete(ctx, p.ID), ErrNotFound)
	_, err = repo.GetByID(ctx, p.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPromoIncrementUsage(t *testing.T) {
	store := newTestStore(t)
	repo := NewPromoRepository(store)
	ctx := context.Background()

	limited, err := repo.Create(ctx, newPromo("LIMITED", 2, nil))
	require.NoError(t, err)
	unlimited, err := repo.Create(ctx, newPromo("UNLIMITED", 0, nil))
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		applied, err := repo.IncrementUsage(ctx, limited.ID)
		require.NoError(t, err)
		assert.True(t, applied, "use %d", i+1)
	}
	applied, err := repo.IncrementUsage(ctx, limited.ID)
	require.NoError(t, err)
	assert.False(t, applied, "limit reached")
	got, err := repo.GetByID(ctx, limited.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, got.UsedCount)

	for i := 0; i < 5; i++ {
		applied, err := repo.IncrementUsage(ctx, unlimited.ID)
		require.NoError(t, err)
		assert.True(t, applied)
	}
	got, err = repo.GetByID(ctx, unlimited.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, got.UsedCount)

	applied, err = repo.IncrementUsage(ctx, "00000000-0000-0000-0000-000000000000")
	require.NoError(t, err)
	assert.False(t, applied, "unknown id")
}

func TestPromoIncrementUsageConcurrent(t *testing.T) {
	store := newTestStore(t)
	repo := NewPromoRepository(store)
	ctx := context.Background()

	p, err := repo.Create(ctx, newPromo("RACE", 3, nil))
	require.NoError(t, err)

	var wg sync.WaitGroup
	var ok atomic.Int64
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			applied, err := repo.IncrementUsage(ctx, p.ID)
			if err == nil && applied {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	assert.EqualValues(t, 3, ok.Load(), "exactly usage_limit increments succeed")
	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, got.UsedCount)
}

func TestSubscriptionRepositoryActive(t *testing.T) {
	store := newTestStore(t)
	plans := NewPlanRepository(store)
	subs := NewSubscriptionRepository(store)
	ctx := context.Background()
	const user = "44444444-4444-4444-4444-444444444444"
	const other = "55555555-5555-5555-5555-555555555555"

	_, err := subs.GetActiveByUser(ctx, user)
	assert.ErrorIs(t, err, ErrNotFound)

	pro, err := plans.Create(ctx, &db.Plan{Name: "Pro", Price: 10, DurationDays: 30, Quotas: map[string]int{"resumes": 10}, Active: true})
	require.NoError(t, err)

	insert := func(uid, status string, ends time.Duration) error {
		_, err := store.Conn(ctx).Exec(ctx,
			`INSERT INTO subscriptions (user_id, plan_id, status, amount, starts_at, ends_at)
			 VALUES ($1, $2, $3, 10, now() - interval '40 days', now() + $4 * interval '1 second')`,
			uid, pro.ID, status, int(ends.Seconds()))
		return err
	}

	require.NoError(t, insert(user, "expired", -time.Hour))
	require.NoError(t, insert(user, "cancelled", time.Hour))
	require.NoError(t, insert(user, "pending", time.Hour))
	_, err = subs.GetActiveByUser(ctx, user)
	assert.ErrorIs(t, err, ErrNotFound, "only active counts")

	require.NoError(t, insert(user, "active", time.Hour))
	assert.Error(t, insert(user, "active", time.Hour), "unique index: one active subscription per user")
	require.NoError(t, insert(other, "active", time.Hour), "another user is unaffected")
	assert.Error(t, insert(other, "bogus", time.Hour), "CHECK status")

	got, err := subs.GetActiveByUser(ctx, user)
	require.NoError(t, err)
	assert.Equal(t, user, got.UserID)
	assert.Equal(t, pro.ID, got.PlanID)
	assert.Nil(t, got.PromoCodeID)
	assert.Equal(t, "active", got.Status)
}

func TestSubscriptionActiveIgnoresLapsed(t *testing.T) {
	store := newTestStore(t)
	plans := NewPlanRepository(store)
	subs := NewSubscriptionRepository(store)
	ctx := context.Background()
	const user = "66666666-6666-6666-6666-666666666666"

	free, err := plans.GetByName(ctx, "Free")
	require.NoError(t, err)
	_, err = store.Conn(ctx).Exec(ctx,
		`INSERT INTO subscriptions (user_id, plan_id, status, amount, starts_at, ends_at)
		 VALUES ($1, $2, 'active', 0, now() - interval '2 days', now() - interval '1 day')`, user, free.ID)
	require.NoError(t, err)
	_, err = subs.GetActiveByUser(ctx, user)
	assert.ErrorIs(t, err, ErrNotFound, "status active but ends_at in the past")
}

func TestSubscriptionLifecycle(t *testing.T) {
	store := newTestStore(t)
	plans := NewPlanRepository(store)
	subs := NewSubscriptionRepository(store)
	ctx := context.Background()
	const user = "77777777-7777-7777-7777-777777777777"

	pro, err := plans.Create(ctx, &db.Plan{Name: "Pro", Price: 10, DurationDays: 30, Quotas: map[string]int{}, Active: true})
	require.NoError(t, err)
	now := time.Now()
	newSub := func() *db.Subscription {
		return &db.Subscription{UserID: user, PlanID: pro.ID, Amount: 10, StartsAt: now, EndsAt: now.Add(30 * 24 * time.Hour)}
	}

	_, err = subs.SetRenewal(ctx, user, false)
	assert.ErrorIs(t, err, ErrNotFound, "nothing to stop")
	_, err = subs.LastEnded(ctx, user)
	assert.ErrorIs(t, err, ErrNotFound)

	created, err := subs.Create(ctx, newSub())
	require.NoError(t, err)
	assert.Equal(t, "active", created.Status)
	_, err = subs.Create(ctx, newSub())
	assert.ErrorIs(t, err, ErrConflict, "one active subscription per user")

	newEnd := created.EndsAt.Add(30 * 24 * time.Hour)
	require.NoError(t, subs.Extend(ctx, created.ID, newEnd))
	got, err := subs.GetActiveByUser(ctx, user)
	require.NoError(t, err)
	assert.WithinDuration(t, newEnd, got.EndsAt, time.Second)
	assert.ErrorIs(t, subs.Extend(ctx, "00000000-0000-0000-0000-000000000000", newEnd), ErrNotFound)

	require.NoError(t, subs.ExpireActive(ctx, user))
	_, err = subs.GetActiveByUser(ctx, user)
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = subs.Create(ctx, newSub())
	require.NoError(t, err, "expiring frees the unique slot")

	// Stopping keeps the subscription active until it ends; stopping twice keeps the first date.
	stopped, err := subs.SetRenewal(ctx, user, false)
	require.NoError(t, err)
	require.NotNil(t, stopped.StoppedAt)
	again, err := subs.SetRenewal(ctx, user, false)
	require.NoError(t, err)
	assert.Equal(t, stopped.StoppedAt.UnixMicro(), again.StoppedAt.UnixMicro())
	got, err = subs.GetActiveByUser(ctx, user)
	require.NoError(t, err, "still active")
	assert.NotNil(t, got.StoppedAt)

	resumed, err := subs.SetRenewal(ctx, user, true)
	require.NoError(t, err)
	assert.Nil(t, resumed.StoppedAt)

	// Paying again resumes a stopped renewal.
	_, err = subs.SetRenewal(ctx, user, false)
	require.NoError(t, err)
	require.NoError(t, subs.Extend(ctx, got.ID, got.EndsAt.Add(24*time.Hour)))
	got, err = subs.GetActiveByUser(ctx, user)
	require.NoError(t, err)
	assert.Nil(t, got.StoppedAt)

	// Once the period is over the subscription is the last ended one, and nothing is left to stop.
	_, err = store.Conn(ctx).Exec(ctx, `UPDATE subscriptions SET starts_at = now() - interval '40 days', ends_at = now() - interval '1 day' WHERE id = $1`, got.ID)
	require.NoError(t, err)
	last, err := subs.LastEnded(ctx, user)
	require.NoError(t, err)
	assert.Equal(t, got.ID, last.ID)
	_, err = subs.SetRenewal(ctx, user, false)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPaymentRepository(t *testing.T) {
	store := newTestStore(t)
	plans := NewPlanRepository(store)
	repo := NewPaymentRepository(store)
	ctx := context.Background()
	const user = "88888888-8888-8888-8888-888888888888"

	free, err := plans.GetByName(ctx, "Free")
	require.NoError(t, err)
	newPayment := func() *db.Payment {
		return &db.Payment{UserID: user, PlanID: free.ID, Provider: "flouci", AmountMillimes: 40000, DiscountMillimes: 10000}
	}

	p, err := repo.Create(ctx, newPayment())
	require.NoError(t, err)
	assert.Nil(t, p.RegularMillimes)
	regular := int64(50000)
	withRegular := newPayment()
	withRegular.RegularMillimes = &regular
	kept, err := repo.Create(ctx, withRegular)
	require.NoError(t, err)
	require.NotNil(t, kept.RegularMillimes)
	assert.EqualValues(t, 50000, *kept.RegularMillimes)
	assert.Equal(t, "pending", p.Status)
	assert.Nil(t, p.ProviderRef)
	assert.EqualValues(t, 40000, p.AmountMillimes)

	bad := newPayment()
	bad.AmountMillimes = -1
	_, err = repo.Create(ctx, bad)
	assert.Error(t, err, "CHECK amount_millimes >= 0")

	_, err = repo.GetByProviderRef(ctx, "flouci", "ref-1")
	assert.ErrorIs(t, err, ErrNotFound)
	require.NoError(t, repo.SetProviderRef(ctx, p.ID, "ref-1"))
	got, err := repo.GetByProviderRef(ctx, "flouci", "ref-1")
	require.NoError(t, err)
	assert.Equal(t, p.ID, got.ID)
	_, err = repo.GetByProviderRef(ctx, "other", "ref-1")
	assert.ErrorIs(t, err, ErrNotFound, "reference is scoped to its provider")

	other, err := repo.Create(ctx, newPayment())
	require.NoError(t, err)
	assert.ErrorIs(t, repo.SetProviderRef(ctx, other.ID, "ref-1"), ErrConflict, "unique provider reference")

	applied, err := repo.MarkCompleted(ctx, p.ID)
	require.NoError(t, err)
	assert.True(t, applied)
	applied, err = repo.MarkCompleted(ctx, p.ID)
	require.NoError(t, err)
	assert.False(t, applied, "replay is a no-op")
	got, err = repo.GetByProviderRef(ctx, "flouci", "ref-1")
	require.NoError(t, err)
	assert.Equal(t, "completed", got.Status)
	assert.NotNil(t, got.CompletedAt)

	require.NoError(t, repo.MarkFailed(ctx, other.ID))
	require.NoError(t, repo.MarkFailed(ctx, p.ID), "completed payments are never failed")
	got, err = repo.GetByProviderRef(ctx, "flouci", "ref-1")
	require.NoError(t, err)
	assert.Equal(t, "completed", got.Status)
}

func TestPaymentReceipts(t *testing.T) {
	store := newTestStore(t)
	plans := NewPlanRepository(store)
	repo := NewPaymentRepository(store)
	ctx := context.Background()
	const user, other = "88888888-8888-8888-8888-888888888888", "99999999-8888-8888-8888-888888888888"

	free, err := plans.GetByName(ctx, "Free")
	require.NoError(t, err)
	pay := func(u string) *db.Payment {
		p, err := repo.Create(ctx, &db.Payment{UserID: u, PlanID: free.ID, Provider: "flouci", AmountMillimes: 35000})
		require.NoError(t, err)
		return p
	}
	first, second, pending, theirs := pay(user), pay(user), pay(user), pay(other)
	assert.Nil(t, first.ReceiptNo, "no receipt number before the payment completes")

	for _, p := range []*db.Payment{first, second, theirs} {
		applied, err := repo.MarkCompleted(ctx, p.ID)
		require.NoError(t, err)
		require.True(t, applied)
	}
	start := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, repo.SetPeriod(ctx, first.ID, start, start.AddDate(0, 0, 30)))
	_, err = store.Conn(ctx).Exec(ctx, `UPDATE payments SET period_end = period_start WHERE id = $1`, first.ID)
	assert.Error(t, err, "a period ends after it starts")

	got, err := repo.Get(ctx, user, first.ID)
	require.NoError(t, err)
	require.NotNil(t, got.ReceiptNo)
	require.NotNil(t, got.PeriodEnd)
	assert.Equal(t, start.AddDate(0, 0, 30), got.PeriodEnd.UTC())
	_, err = repo.Get(ctx, other, first.ID)
	assert.ErrorIs(t, err, ErrNotFound, "another user's payment")

	list, err := repo.ListCompletedByUser(ctx, user, 10)
	require.NoError(t, err)
	require.Len(t, list, 2, "completed payments of this user only")
	for _, p := range list {
		assert.NotEqual(t, pending.ID, p.ID)
		require.NotNil(t, p.ReceiptNo)
	}
	assert.NotEqual(t, *list[0].ReceiptNo, *list[1].ReceiptNo, "receipt numbers are unique")

	list, err = repo.ListCompletedByUser(ctx, user, 1)
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestIssuerRepository(t *testing.T) {
	store := newTestStore(t)
	repo := NewIssuerRepository(store)
	ctx := context.Background()

	empty, err := repo.Get(ctx)
	require.NoError(t, err)
	assert.Empty(t, empty.LegalName)
	assert.Empty(t, empty.TaxID)
	assert.Nil(t, empty.UpdatedBy)

	const admin = "0b8f9a52-6d0e-4e0a-9d43-2f7c1e0d5a11"
	out, err := repo.Update(ctx, admin, &db.ReceiptIssuer{LegalName: "Example SARL", TaxID: "0000000/A/A/M/000"})
	require.NoError(t, err)
	assert.Equal(t, "Example SARL", out.LegalName)
	assert.Empty(t, out.Address)
	require.NotNil(t, out.UpdatedBy)
	assert.Equal(t, admin, *out.UpdatedBy)

	again, err := repo.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, "0000000/A/A/M/000", again.TaxID)

	var rows int
	require.NoError(t, store.Conn(ctx).QueryRow(ctx, `SELECT count(*) FROM receipt_issuer`).Scan(&rows))
	assert.Equal(t, 1, rows, "the table holds one row")
	_, err = store.Conn(ctx).Exec(ctx, `INSERT INTO receipt_issuer (id) VALUES (false)`)
	assert.Error(t, err, "a second row is refused")
}

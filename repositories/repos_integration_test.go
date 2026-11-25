//go:build integration

package repositories

import (
	"context"
	"fmt"
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
	assert.Equal(t, map[string]int{"emails_per_month": 0, "resumes": 1, "cv_downloads": 0, "ats_experience": 0, "ats_education": 0,
		"ats_skills": 0, "ats_layout": 0, "ats_file": 0}, free.Quotas, "no download and a part of the diagnosis without a subscription")

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

func TestRenewalReminders(t *testing.T) {
	store := newTestStore(t)
	plans := NewPlanRepository(store)
	repo := NewSubscriptionRepository(store)
	ctx := context.Background()
	week := 7 * 24 * time.Hour

	paid, err := plans.Create(ctx, &db.Plan{Name: "Mensuel", Price: 35, DurationDays: 30, Quotas: map[string]int{}, Active: true})
	require.NoError(t, err)
	free, err := plans.GetByName(ctx, "Free")
	require.NoError(t, err)

	// add inserts a subscription ending in `in` (negative: already ended) and returns its id.
	n := 0
	add := func(planID string, in time.Duration, status string, stopped bool) string {
		n++
		user := fmt.Sprintf("00000000-0000-0000-0000-%012d", n)
		var id string
		require.NoError(t, store.Conn(ctx).QueryRow(ctx, `
			INSERT INTO subscriptions (user_id, plan_id, status, amount, starts_at, ends_at, stopped_at)
			VALUES ($1, $2, $3, 35, now() - interval '30 days', now() + make_interval(secs => $4), CASE WHEN $5 THEN now() END) RETURNING id`,
			user, planID, status, in.Seconds(), stopped).Scan(&id))
		return id
	}
	ids := func(rs []db.RenewalReminder) []string {
		out := make([]string, 0, len(rs))
		for _, r := range rs {
			out = append(out, r.SubscriptionID)
		}
		return out
	}

	soon := add(paid.ID, 3*24*time.Hour, "active", false)
	add(paid.ID, 20*24*time.Hour, "active", false) // too early
	add(paid.ID, 3*24*time.Hour, "active", true)   // the student stopped it: nothing to renew
	add(free.ID, 3*24*time.Hour, "active", false)  // a free plan has nothing to pay
	lapsed := add(paid.ID, -2*24*time.Hour, "active", false)
	add(paid.ID, -20*24*time.Hour, "active", false) // lapsed too long ago
	add(paid.ID, -2*24*time.Hour, "active", true)   // stopped, then ended: as the student wanted
	add(paid.ID, -2*24*time.Hour, "cancelled", false)

	due, err := repo.ClaimReminders(ctx, db.ReminderDue, week, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{soon}, ids(due))
	assert.Equal(t, "Mensuel", due[0].PlanName)
	assert.InDelta(t, 35, due[0].PlanPrice, 0.001)

	again, err := repo.ClaimReminders(ctx, db.ReminderDue, week, 10)
	require.NoError(t, err)
	assert.Empty(t, again, "one reminder per period")

	// A reminder that could not be raised is given back and claimed again.
	require.NoError(t, repo.ReleaseReminder(ctx, soon, db.ReminderDue))
	retry, err := repo.ClaimReminders(ctx, db.ReminderDue, week, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{soon}, ids(retry))

	// Renewing moves the end of the period: when that new period nears its end, it gets its own reminder.
	require.NoError(t, repo.Extend(ctx, soon, time.Now().Add(33*24*time.Hour)))
	none, err := repo.ClaimReminders(ctx, db.ReminderDue, week, 10)
	require.NoError(t, err)
	assert.Empty(t, none)
	require.NoError(t, repo.Extend(ctx, soon, time.Now().Add(2*24*time.Hour)))
	next, err := repo.ClaimReminders(ctx, db.ReminderDue, week, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{soon}, ids(next))

	gone, err := repo.ClaimReminders(ctx, db.ReminderLapsed, week, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{lapsed}, ids(gone))
	gone, err = repo.ClaimReminders(ctx, db.ReminderLapsed, week, 10)
	require.NoError(t, err)
	assert.Empty(t, gone)

	// Someone who paid again after a lapse has a running subscription: the old one is not chased.
	var user string
	back := add(paid.ID, -1*24*time.Hour, "expired", false)
	require.NoError(t, store.Conn(ctx).QueryRow(ctx, `SELECT user_id FROM subscriptions WHERE id = $1`, back).Scan(&user))
	_, err = store.Conn(ctx).Exec(ctx, `INSERT INTO subscriptions (user_id, plan_id, status, amount, starts_at, ends_at) VALUES ($1, $2, 'active', 35, now(), now() + interval '30 days')`, user, paid.ID)
	require.NoError(t, err)
	gone, err = repo.ClaimReminders(ctx, db.ReminderLapsed, week, 10)
	require.NoError(t, err)
	assert.Empty(t, gone)

	_, err = repo.ClaimReminders(ctx, "later", week, 10)
	assert.Error(t, err)
	assert.Error(t, repo.ReleaseReminder(ctx, soon, "later"))

	// The limit bounds one call; the rest is taken at the next.
	a, b := add(paid.ID, 24*time.Hour, "active", false), add(paid.ID, 36*time.Hour, "active", false)
	one, err := repo.ClaimReminders(ctx, db.ReminderDue, week, 1)
	require.NoError(t, err)
	assert.Equal(t, []string{a}, ids(one), "the period that ends first")
	two, err := repo.ClaimReminders(ctx, db.ReminderDue, week, 1)
	require.NoError(t, err)
	assert.Equal(t, []string{b}, ids(two))
}

func TestPlanLaunchEnd(t *testing.T) {
	store := newTestStore(t)
	plans := NewPlanRepository(store)
	subs := NewSubscriptionRepository(store)
	ctx := context.Background()
	regular := 50.0
	last := time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)

	p, err := plans.Create(ctx, &db.Plan{Name: "Lancement", Price: 35, RegularPrice: &regular, LaunchEndsOn: &last, DurationDays: 30, Quotas: map[string]int{}, Active: true})
	require.NoError(t, err)
	require.NotNil(t, p.LaunchEndsOn)
	assert.Equal(t, "2027-01-31", p.LaunchEndsOn.Format(time.DateOnly))

	got, err := plans.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "2027-01-31", got.LaunchEndsOn.Format(time.DateOnly))

	// Updating without a day removes the end.
	got, err = plans.Update(ctx, p.ID, &db.Plan{Name: "Lancement", Price: 35, RegularPrice: &regular, DurationDays: 30, Quotas: map[string]int{}, Active: true})
	require.NoError(t, err)
	assert.Nil(t, got.LaunchEndsOn)

	// An end without a regular price to go back to is refused by the table too.
	_, err = plans.Update(ctx, p.ID, &db.Plan{Name: "Lancement", Price: 35, LaunchEndsOn: &last, DurationDays: 30, Quotas: map[string]int{}, Active: true})
	assert.Error(t, err)

	// A reminder names what the renewal will cost: the regular price when the period ends after the launch.
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour)
	inTenDays := time.Now().UTC().AddDate(0, 0, 10).Truncate(24 * time.Hour)
	over, err := plans.Create(ctx, &db.Plan{Name: "Fini", Price: 35, RegularPrice: &regular, LaunchEndsOn: &yesterday, DurationDays: 30, Quotas: map[string]int{}, Active: true})
	require.NoError(t, err)
	still, err := plans.Create(ctx, &db.Plan{Name: "En cours", Price: 35, RegularPrice: &regular, LaunchEndsOn: &inTenDays, DurationDays: 30, Quotas: map[string]int{}, Active: true})
	require.NoError(t, err)
	for i, planID := range []string{over.ID, still.ID} {
		_, err := store.Conn(ctx).Exec(ctx, `INSERT INTO subscriptions (user_id, plan_id, status, amount, starts_at, ends_at)
			VALUES ($1, $2, 'active', 35, now() - interval '27 days', now() + interval '3 days')`, fmt.Sprintf("00000000-0000-0000-0000-%012d", 900+i), planID)
		require.NoError(t, err)
	}
	due, err := subs.ClaimReminders(ctx, db.ReminderDue, 7*24*time.Hour, 10)
	require.NoError(t, err)
	prices := map[string]float64{}
	for _, r := range due {
		prices[r.PlanID] = r.PlanPrice
	}
	assert.Equal(t, map[string]float64{over.ID: 50, still.ID: 35}, prices)
}

func TestGifts(t *testing.T) {
	store := newTestStore(t)
	plans := NewPlanRepository(store)
	subs := NewSubscriptionRepository(store)
	gifts := NewGiftRepository(store)
	ctx := context.Background()
	const user = "88888888-8888-8888-8888-888888888888"
	const other = "99999999-9999-9999-9999-999999999999"
	const admin = "00000000-0000-4000-8000-000000000001"

	pro, err := plans.Create(ctx, &db.Plan{Name: "Pro", Price: 10, DurationDays: 30, Quotas: map[string]int{}, Active: true})
	require.NoError(t, err)
	now := time.Now().Truncate(time.Microsecond)

	// A subscription given by the team is stopped from the start.
	given, err := subs.Create(ctx, &db.Subscription{UserID: other, PlanID: pro.ID, StartsAt: now, EndsAt: now.AddDate(0, 0, 30), StoppedAt: &now})
	require.NoError(t, err)
	require.NotNil(t, given.StoppedAt)
	assert.WithinDuration(t, now, *given.StoppedAt, time.Millisecond)

	// Days added to a running subscription leave its renewal as it was.
	sub, err := subs.Create(ctx, &db.Subscription{UserID: user, PlanID: pro.ID, Amount: 10, StartsAt: now, EndsAt: now.AddDate(0, 0, 10)})
	require.NoError(t, err)
	_, err = subs.SetRenewal(ctx, user, false)
	require.NoError(t, err)
	moved, err := subs.AddDays(ctx, sub.ID, 30)
	require.NoError(t, err)
	assert.WithinDuration(t, sub.EndsAt.AddDate(0, 0, 30), moved.EndsAt, time.Hour+time.Second, "30 days later (an hour apart at most on a clock change)")
	assert.NotNil(t, moved.StoppedAt, "still stopped")
	_, err = subs.AddDays(ctx, "00000000-0000-0000-0000-000000000000", 30)
	assert.ErrorIs(t, err, ErrNotFound)

	none, err := gifts.ListByUser(ctx, user, 10)
	require.NoError(t, err)
	assert.Empty(t, none)

	first, err := gifts.Create(ctx, &db.Gift{UserID: user, SubscriptionID: sub.ID, PlanID: pro.ID, Days: 30, GivenBy: admin,
		PeriodStart: sub.EndsAt, PeriodEnd: moved.EndsAt})
	require.NoError(t, err)
	assert.NotEmpty(t, first.ID)
	assert.Equal(t, admin, first.GivenBy)
	second, err := gifts.Create(ctx, &db.Gift{UserID: user, SubscriptionID: sub.ID, PlanID: pro.ID, Days: 30, GivenBy: admin,
		PeriodStart: moved.EndsAt, PeriodEnd: moved.EndsAt.AddDate(0, 0, 30)})
	require.NoError(t, err)
	_, err = gifts.Create(ctx, &db.Gift{UserID: other, SubscriptionID: given.ID, PlanID: pro.ID, Days: 30, GivenBy: admin,
		PeriodStart: now, PeriodEnd: now.AddDate(0, 0, 30)})
	require.NoError(t, err)

	list, err := gifts.ListByUser(ctx, user, 10)
	require.NoError(t, err)
	require.Len(t, list, 2, "only this student's")
	assert.ElementsMatch(t, []string{first.ID, second.ID}, []string{list[0].ID, list[1].ID})
	one, err := gifts.ListByUser(ctx, user, 1)
	require.NoError(t, err)
	assert.Len(t, one, 1)

	_, err = gifts.Create(ctx, &db.Gift{UserID: user, SubscriptionID: sub.ID, PlanID: pro.ID, Days: 0, GivenBy: admin,
		PeriodStart: now, PeriodEnd: now.AddDate(0, 0, 30)})
	assert.Error(t, err, "a gift gives at least a day")

	// The lock is held until the transaction ends: a second gift to the same student waits for the first.
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- store.WithTx(ctx, func(ctx context.Context) error {
			if err := gifts.LockUser(ctx, user); err != nil {
				return err
			}
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	waited := make(chan error, 1)
	go func() {
		waited <- store.WithTx(ctx, func(ctx context.Context) error { return gifts.LockUser(ctx, user) })
	}()
	require.NoError(t, store.WithTx(ctx, func(ctx context.Context) error { return gifts.LockUser(ctx, other) }), "another student is not held up")
	select {
	case err := <-waited:
		t.Fatalf("the second gift did not wait for the first (err: %v)", err)
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-done)
	require.NoError(t, <-waited)
}

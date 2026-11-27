//go:build integration

package services

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"9itchi-payments-api/core"
	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/repositories"
	"9itchi-payments-api/services/mocks"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	igAdminURL string
	igDBSeq    atomic.Int64
)

// TestMain boots one embedded PostgreSQL for the package (no Docker needed). Run: go test -tags integration ./services/...
func TestMain(m *testing.M) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	dataDir, _ := os.MkdirTemp("", "itchi-pg-data-")
	runDir, _ := os.MkdirTemp("", "itchi-pg-run-")
	cfg := embeddedpostgres.DefaultConfig().
		Version(embeddedpostgres.V16).Port(uint32(port)).DataPath(dataDir).RuntimePath(runDir)
	if bin := os.Getenv("PG_BINARIES"); bin != "" { // use local PostgreSQL 16 binaries instead of downloading them
		cfg = cfg.BinariesPath(bin)
	}
	pg := embeddedpostgres.NewDatabase(cfg)
	if err := pg.Start(); err != nil {
		fmt.Println("embedded postgres:", err)
		os.Exit(1)
	}
	igAdminURL = fmt.Sprintf("postgres://postgres:postgres@localhost:%d/postgres?sslmode=disable", port)
	code := m.Run()
	_ = pg.Stop()
	os.RemoveAll(dataDir)
	os.RemoveAll(runDir)
	os.Exit(code)
}

// igNewStore creates an isolated, migrated database for one test.
func igNewStore(t *testing.T) *repositories.Store {
	t.Helper()
	ctx := context.Background()
	name := fmt.Sprintf("t_%d", igDBSeq.Add(1))
	admin, err := pgx.Connect(ctx, igAdminURL)
	require.NoError(t, err)
	defer admin.Close(ctx)
	_, err = admin.Exec(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)
	url := fmt.Sprintf("%s/%s?sslmode=disable", igAdminURL[:len(igAdminURL)-len("/postgres?sslmode=disable")], name)
	require.NoError(t, repositories.RunMigrations(ctx, url))
	store, err := repositories.NewStore(ctx, url)
	require.NoError(t, err)
	t.Cleanup(store.Close)
	return store
}

// igProvider is a fake gateway: a payment is paid when the test says so.
type igProvider struct {
	mu      sync.Mutex
	paid    map[string]bool
	created int
	amounts map[string]int64
}

func (p *igProvider) Available() bool { return true }

func (p *igProvider) CreateCheckout(_ context.Context, req core.CheckoutRequest) (core.CheckoutResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.created++
	ref := "ref-" + req.PaymentID
	p.amounts[ref] = req.AmountMillimes
	return core.CheckoutResult{Ref: ref, URL: "https://pay.example/" + ref}, nil
}

func (p *igProvider) VerifyWebhook(payload []byte, signature string) (string, error) {
	if signature != "ok" {
		return "", core.ErrInvalidSignature
	}
	return string(payload), nil
}

func (p *igProvider) VerifyPayment(_ context.Context, ref string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paid[ref], nil
}

func (p *igProvider) pay(paymentID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paid["ref-"+paymentID] = true
}

// igClock is the frozen, movable time every service of an env reads.
type igClock struct{ t time.Time }

func (c *igClock) Now() time.Time { return c.t }

type igEnv struct {
	clock    *igClock
	provider *igProvider
	repos    *repositories.Repositories
	plans    PlanService
	promos   PromoService
	sales    SaleService
	subs     SubscriptionService
}

// igNewEnv wires real repositories and services like InitServices does, with a fake gateway and a frozen clock.
// The clock starts at the real time (cut to the second): the database decides "active" with its own now().
func igNewEnv(t *testing.T) *igEnv {
	t.Helper()
	store := igNewStore(t)
	repos := &repositories.Repositories{
		Store:        store,
		Plan:         repositories.NewPlanRepository(store),
		Promo:        repositories.NewPromoRepository(store),
		Subscription: repositories.NewSubscriptionRepository(store),
		Payment:      repositories.NewPaymentRepository(store),
		Sale:         repositories.NewSaleRepository(store),
	}
	clock := &igClock{t: time.Now().UTC().Truncate(time.Second)}
	fake := &igProvider{paid: map[string]bool{}, amounts: map[string]int64{}}
	providers := core.ProviderFactory{core.FlouciProviderName: func() core.PaymentProvider { return fake }}
	priced := PricedPlans(repos.Plan, repos.Sale, clock.Now)

	promos := NewPromoService(repos.Promo, priced).(*promoService)
	promos.now = clock.Now
	sales := NewSaleService(repos.Sale, repos.Plan).(*saleService)
	sales.now = clock.Now
	subs := NewSubscriptionService(repos.Subscription, priced, repos.Promo, repos.Student, repos.Payment, repos.Store, providers, &mocks.Publisher{}).(*subscriptionService)
	subs.now = clock.Now
	return &igEnv{clock: clock, provider: fake, repos: repos, promos: promos, sales: sales, subs: subs,
		plans: NewPlanService(repos.Plan, priced, repos.Subscription)}
}

const igUserB = "44444444-3333-4333-8333-333333333333"

func igOK[T any](t *testing.T, v *T, apiErr *models.ApiError) *T {
	t.Helper()
	require.Nil(t, apiErr, "unexpected api error")
	require.NotNil(t, v)
	return v
}

func igCode(t *testing.T, apiErr *models.ApiError, code int) {
	t.Helper()
	require.NotNil(t, apiErr, "expected an error with status %d", code)
	assert.Equal(t, code, apiErr.Code, apiErr.Error.Error())
}

func igSameTime(t *testing.T, want time.Time, got *time.Time, msg string) {
	t.Helper()
	require.NotNil(t, got, msg)
	assert.True(t, want.Equal(*got), "%s: want %s got %s", msg, want, *got)
}

func igDay(t time.Time) string { return today(t).Format(time.DateOnly) }

func (e *igEnv) plan(t *testing.T, name string, price float64, regular *float64, days int) api.PlanResponse {
	t.Helper()
	ends := ""
	if regular != nil {
		ends = igDay(e.clock.t.AddDate(0, 0, 60))
	}
	p, apiErr := e.plans.CreatePlan(context.Background(), adminID, api.CreatePlanRequest{Name: name, Price: price, RegularPrice: regular,
		LaunchEndsOn: ends, DurationDays: days, Quotas: map[string]int{"resumes": 5}})
	return *igOK(t, p, apiErr)
}

func (e *igEnv) promo(t *testing.T, code, typ string, value float64, from, until time.Time, limit int) api.PromoResponse {
	t.Helper()
	p, apiErr := e.promos.CreatePromo(context.Background(), adminID, api.CreatePromoRequest{Code: code, Type: typ, Value: value,
		ValidFrom: from, ValidUntil: until, UsageLimit: limit})
	return *igOK(t, p, apiErr)
}

func (e *igEnv) checkout(user, planID, code string) (*api.CheckoutResponse, *models.ApiError) {
	return e.subs.Checkout(context.Background(), user, api.CheckoutRequest{PlanID: planID, PromoCode: code})
}

// buy checks out and completes the payment through the verify path.
func (e *igEnv) buy(t *testing.T, user, planID, code string) api.PaymentResponse {
	t.Helper()
	res := igWrap(e.checkout(user, planID, code)).ok(t)
	e.provider.pay(res.PaymentID)
	p, apiErr := e.subs.VerifyPayment(context.Background(), user, res.PaymentID)
	return *igOK(t, p, apiErr)
}

func (e *igEnv) mine(t *testing.T, user string) *api.MySubscriptionResponse {
	t.Helper()
	r, apiErr := e.subs.MySubscription(context.Background(), user)
	return igOK(t, r, apiErr)
}

func (e *igEnv) usedCount(t *testing.T, code string) int {
	t.Helper()
	l, apiErr := e.promos.ListPromos(context.Background(), adminID)
	igOK(t, l, apiErr)
	for _, p := range l.Items {
		if p.Code == code {
			return p.UsedCount
		}
	}
	t.Fatalf("promo %s not listed", code)
	return -1
}

func igHasPlan(l *api.PlanListResponse, id string) bool {
	for _, p := range l.Items {
		if p.ID == id {
			return true
		}
	}
	return false
}

func TestBillingFlowIntegration(t *testing.T) {
	ctx := context.Background()
	regular50 := func() *float64 { v := 50.0; return &v }

	t.Run("admin creates a paid plan", func(t *testing.T) {
		e := igNewEnv(t)
		p := e.plan(t, "Pro Mensuel", 35, regular50(), 30)
		assert.Equal(t, 35.0, p.Price)
		require.NotNil(t, p.RegularPrice)
		assert.Equal(t, 50.0, *p.RegularPrice)
		assert.Equal(t, igDay(e.clock.t.AddDate(0, 0, 60)), p.LaunchEndsOn)
		assert.True(t, p.Active)

		list := igWrap(e.plans.ListPlans(ctx)).ok(t)
		assert.True(t, igHasPlan(list, p.ID))

		for _, name := range []string{"Free", "free", "FREE", " fReE "} {
			_, apiErr := e.plans.CreatePlan(ctx, adminID, api.CreatePlanRequest{Name: name, Price: 10, DurationDays: 30, Quotas: map[string]int{"resumes": 1}})
			igCode(t, apiErr, http.StatusBadRequest)
		}
		_, apiErr := e.plans.CreatePlan(ctx, adminID, api.CreatePlanRequest{Name: "Zero", Price: 0, DurationDays: 30, Quotas: map[string]int{"resumes": 1}})
		igCode(t, apiErr, http.StatusBadRequest)
		_, apiErr = e.plans.CreatePlan(ctx, adminID, api.CreatePlanRequest{Name: "Pro Mensuel", Price: 20, DurationDays: 30, Quotas: map[string]int{"resumes": 1}})
		igCode(t, apiErr, http.StatusConflict)
		_, apiErr = e.plans.CreatePlan(ctx, adminID, api.CreatePlanRequest{Name: "pro MENSUEL", Price: 20, DurationDays: 30, Quotas: map[string]int{"resumes": 1}})
		igCode(t, apiErr, http.StatusConflict)
	})

	t.Run("checkout with percent and fixed promo then completion", func(t *testing.T) {
		e := igNewEnv(t)
		plan := e.plan(t, "Pro Mensuel", 35, regular50(), 30)
		from, until := e.clock.t.AddDate(0, 0, -1), e.clock.t.AddDate(0, 0, 30)
		e.promo(t, "SAVE20", "percentage", 20, from, until, 0)
		e.promo(t, "MINUS5", "fixed", 5, from, until, 0)
		end := e.clock.t.AddDate(0, 0, 30)

		// percent promo, completed by the webhook
		res := igWrap(e.checkout(userID, plan.ID, "save20")).ok(t)
		assert.NotEmpty(t, res.PaymentURL)
		assert.Equal(t, int64(28000), e.provider.amounts["ref-"+res.PaymentID])
		pending := igWrap(e.subs.VerifyPayment(ctx, userID, res.PaymentID)).ok(t)
		assert.Equal(t, "pending", pending.Status, "unpaid payment stays pending")
		assert.True(t, e.mine(t, userID).IsFree)

		e.provider.pay(res.PaymentID)
		ack, apiErr := e.subs.Webhook(ctx, core.FlouciProviderName, []byte("ref-"+res.PaymentID), "ok")
		igOK(t, ack, apiErr)
		_, apiErr = e.subs.Webhook(ctx, core.FlouciProviderName, []byte("ref-"+res.PaymentID), "bad")
		igCode(t, apiErr, http.StatusUnauthorized)

		pay := igWrap(e.subs.VerifyPayment(ctx, userID, res.PaymentID)).ok(t)
		assert.Equal(t, "completed", pay.Status)
		assert.Equal(t, int64(28000), pay.AmountMillimes)
		assert.Equal(t, int64(7000), pay.DiscountMillimes)
		assert.Equal(t, "SAVE20", pay.PromoCode)
		assert.Regexp(t, `^\d{4}-\d{6}$`, pay.ReceiptNo)
		igSameTime(t, e.clock.t, pay.PeriodStart, "period start")
		igSameTime(t, end, pay.PeriodEnd, "period end")
		my := e.mine(t, userID)
		assert.False(t, my.IsFree)
		require.NotNil(t, my.Subscription)
		assert.Equal(t, "active", my.Subscription.Status)
		assert.Equal(t, 28.0, my.Subscription.Amount)
		assert.True(t, e.clock.t.Equal(my.Subscription.StartsAt))
		assert.True(t, end.Equal(my.Subscription.EndsAt))
		assert.Equal(t, 1, e.usedCount(t, "SAVE20"))

		// replaying completion changes nothing
		_, apiErr = e.subs.Webhook(ctx, core.FlouciProviderName, []byte("ref-"+res.PaymentID), "ok")
		require.Nil(t, apiErr)
		again := igWrap(e.subs.VerifyPayment(ctx, userID, res.PaymentID)).ok(t)
		assert.Equal(t, pay.ReceiptNo, again.ReceiptNo)
		assert.True(t, end.Equal(e.mine(t, userID).Subscription.EndsAt))
		assert.Equal(t, 1, e.usedCount(t, "SAVE20"))
		list := igWrap(e.subs.ListPayments(ctx, userID)).ok(t)
		assert.Len(t, list.Items, 1)

		// fixed promo, completed by verify
		fixed := e.buy(t, igUserB, plan.ID, "MINUS5")
		assert.Equal(t, "completed", fixed.Status)
		assert.Equal(t, int64(30000), fixed.AmountMillimes)
		assert.Equal(t, int64(5000), fixed.DiscountMillimes)
		assert.Regexp(t, `^\d{4}-\d{6}$`, fixed.ReceiptNo)
		assert.True(t, end.Equal(e.mine(t, igUserB).Subscription.EndsAt))
		assert.Equal(t, 1, e.usedCount(t, "MINUS5"))
		assert.Equal(t, 1, e.usedCount(t, "SAVE20"))
	})

	t.Run("promo limits", func(t *testing.T) {
		e := igNewEnv(t)
		plan := e.plan(t, "Pro Mensuel", 35, regular50(), 30)
		now := e.clock.t
		e.promo(t, "OLD", "percentage", 10, now.AddDate(0, 0, -10), now.AddDate(0, 0, -1), 0)
		e.promo(t, "LATER", "percentage", 10, now.AddDate(0, 0, 1), now.AddDate(0, 0, 10), 0)
		e.promo(t, "ONCE", "percentage", 10, now.AddDate(0, 0, -1), now.AddDate(0, 0, 10), 1)

		_, apiErr := e.checkout(userID, plan.ID, "OLD")
		igCode(t, apiErr, http.StatusBadRequest)
		assert.Contains(t, apiErr.Error.Error(), "expired")
		_, apiErr = e.checkout(userID, plan.ID, "LATER")
		igCode(t, apiErr, http.StatusBadRequest)
		assert.Contains(t, apiErr.Error.Error(), "not yet valid")
		_, apiErr = e.checkout(userID, plan.ID, "NOPE")
		igCode(t, apiErr, http.StatusBadRequest)

		paid := e.buy(t, userID, plan.ID, "ONCE")
		assert.Equal(t, int64(31500), paid.AmountMillimes)
		_, apiErr = e.checkout(igUserB, plan.ID, "ONCE")
		igCode(t, apiErr, http.StatusBadRequest)
		assert.Contains(t, apiErr.Error.Error(), "usage limit")
		assert.Equal(t, 1, e.usedCount(t, "ONCE"))
		assert.Equal(t, 1, e.provider.created, "only the first valid checkout reached the gateway")
	})

	t.Run("sale and promo code", func(t *testing.T) {
		e := igNewEnv(t)
		plan := e.plan(t, "Pro Mensuel", 35, regular50(), 30)
		now := e.clock.t
		e.promo(t, "SAVE20", "percentage", 20, now.AddDate(0, 0, -1), now.AddDate(0, 0, 30), 0)
		saleReq := api.SaleRequest{Name: "Rentree", Percent: 20, StartsOn: igDay(now), EndsOn: igDay(now.AddDate(0, 0, 5))}
		sale := igWrap(e.sales.CreateSale(ctx, saleReq)).ok(t)

		// 20% of the regular 50 is 40: above the launch price 35, a sale never raises the price
		res := igWrap(e.checkout(userID, plan.ID, "")).ok(t)
		assert.Equal(t, int64(35000), e.provider.amounts["ref-"+res.PaymentID])

		saleReq.Percent = 40
		igWrap(e.sales.UpdateSale(ctx, sale.ID, saleReq)).ok(t)
		// 40% of the regular 50 is 30: below the launch price
		listed := igWrap(e.plans.ListPlans(ctx)).ok(t)
		for _, p := range listed.Items {
			if p.ID == plan.ID {
				assert.Equal(t, 30.0, p.Price)
				require.NotNil(t, p.RegularPrice)
				assert.Equal(t, 50.0, *p.RegularPrice)
				require.NotNil(t, p.Sale)
				assert.Equal(t, "Rentree", p.Sale.Name)
			}
		}
		paid := e.buy(t, userID, plan.ID, "")
		assert.Equal(t, int64(30000), paid.AmountMillimes)
		require.NotNil(t, paid.RegularMillimes)
		assert.Equal(t, int64(50000), *paid.RegularMillimes)
		assert.Equal(t, "Rentree", paid.SaleName)

		_, apiErr := e.checkout(igUserB, plan.ID, "SAVE20")
		igCode(t, apiErr, http.StatusBadRequest)
		assert.Equal(t, reasonDuringSale, apiErr.Error.Error())
		assert.Equal(t, 0, e.usedCount(t, "SAVE20"))

		// the sale is over the next week: the launch price is back
		e.clock.t = now.AddDate(0, 0, 7)
		listed = igWrap(e.plans.ListPlans(ctx)).ok(t)
		for _, p := range listed.Items {
			if p.ID == plan.ID {
				assert.Equal(t, 35.0, p.Price)
				assert.Nil(t, p.Sale)
			}
		}
	})

	t.Run("deactivated plan", func(t *testing.T) {
		e := igNewEnv(t)
		plan := e.plan(t, "Pro Mensuel", 35, regular50(), 30)
		e.buy(t, userID, plan.ID, "")
		sub := e.mine(t, userID).Subscription
		require.NotNil(t, sub)

		// A held plan keeps its length (quota periods of its subscribers are counted with it); prices still change.
		edit := api.UpdatePlanRequest{Name: plan.Name, Price: 30, RegularPrice: regular50(), DurationDays: 365, Quotas: plan.Quotas}
		_, apiErr := e.plans.UpdatePlan(ctx, adminID, plan.ID, edit)
		igCode(t, apiErr, http.StatusConflict)
		edit.DurationDays = 30
		_, apiErr = e.plans.UpdatePlan(ctx, adminID, plan.ID, edit)
		require.Nil(t, apiErr)

		_, apiErr = e.plans.DeactivatePlan(ctx, adminID, plan.ID)
		require.Nil(t, apiErr)
		list := igWrap(e.plans.ListPlans(ctx)).ok(t)
		assert.False(t, igHasPlan(list, plan.ID))
		all := igWrap(e.plans.ListAllPlans(ctx)).ok(t)
		assert.True(t, igHasPlan(all, plan.ID))

		_, apiErr = e.checkout(igUserB, plan.ID, "")
		igCode(t, apiErr, http.StatusNotFound)

		my := e.mine(t, userID)
		assert.False(t, my.IsFree, "subscriber keeps access")
		require.NotNil(t, my.Subscription)
		assert.Equal(t, plan.ID, my.Subscription.PlanID)
		assert.True(t, sub.EndsAt.Equal(my.Subscription.EndsAt))
		assert.True(t, e.clock.t.AddDate(0, 0, 30).Equal(my.Subscription.EndsAt))
	})

	t.Run("plan change and renewal", func(t *testing.T) {
		e := igNewEnv(t)
		monthly := e.plan(t, "Pro Mensuel", 35, regular50(), 30)
		yearlyRegular := 400.0
		yearly := e.plan(t, "Pro Annuel", 300, &yearlyRegular, 365)
		t0 := e.clock.t

		e.buy(t, userID, monthly.ID, "")
		old := e.mine(t, userID).Subscription
		require.NotNil(t, old)
		assert.True(t, t0.AddDate(0, 0, 30).Equal(old.EndsAt))

		// 10 days later: 20 monthly days are unused and carried over
		e.clock.t = t0.AddDate(0, 0, 10)
		now := e.clock.t
		pay := e.buy(t, userID, yearly.ID, "")
		assert.Equal(t, int64(300000), pay.AmountMillimes)
		my := e.mine(t, userID)
		require.NotNil(t, my.Subscription)
		assert.NotEqual(t, old.ID, my.Subscription.ID)
		assert.Equal(t, yearly.ID, my.Subscription.PlanID)
		wantEnd := now.AddDate(0, 0, 365).Add(20 * 24 * time.Hour)
		assert.True(t, wantEnd.Equal(my.Subscription.EndsAt), "want %s got %s", wantEnd, my.Subscription.EndsAt)
		assert.True(t, now.Equal(my.Subscription.StartsAt))
		igSameTime(t, now, pay.PeriodStart, "period start")
		igSameTime(t, wantEnd, pay.PeriodEnd, "period end")
		active, err := e.repos.Subscription.GetActiveByUser(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, my.Subscription.ID, active.ID, "the monthly subscription is no longer active")

		// same plan again: extends from the current end, not from now
		e.buy(t, igUserB, monthly.ID, "")
		e.clock.t = now.AddDate(0, 0, 5)
		renewed := e.buy(t, igUserB, monthly.ID, "")
		bEnd := now.AddDate(0, 0, 30)
		igSameTime(t, bEnd, renewed.PeriodStart, "renewal period start")
		igSameTime(t, bEnd.AddDate(0, 0, 30), renewed.PeriodEnd, "renewal period end")
		assert.True(t, bEnd.AddDate(0, 0, 30).Equal(e.mine(t, igUserB).Subscription.EndsAt))
	})

	t.Run("zero TND checkout skips the gateway", func(t *testing.T) {
		e := igNewEnv(t)
		plan := e.plan(t, "Pro Mensuel", 35, regular50(), 30)
		now := e.clock.t
		e.promo(t, "FREE100", "percentage", 100, now.AddDate(0, 0, -1), now.AddDate(0, 0, 30), 0)

		res := igWrap(e.checkout(userID, plan.ID, "free100")).ok(t)
		assert.Empty(t, res.PaymentURL)
		assert.Equal(t, 0, e.provider.created, "the gateway is never called")
		pay := igWrap(e.subs.VerifyPayment(ctx, userID, res.PaymentID)).ok(t)
		assert.Equal(t, "completed", pay.Status)
		assert.Equal(t, "free", pay.Provider)
		assert.Equal(t, int64(0), pay.AmountMillimes)
		assert.Equal(t, int64(35000), pay.DiscountMillimes)
		assert.Regexp(t, `^\d{4}-\d{6}$`, pay.ReceiptNo)
		my := e.mine(t, userID)
		assert.False(t, my.IsFree)
		assert.True(t, now.AddDate(0, 0, 30).Equal(my.Subscription.EndsAt))
		assert.Equal(t, 1, e.usedCount(t, "FREE100"))
	})
}

// igR carries a service result so that calls read igWrap(svc.Method(...)).ok(t).
type igR[T any] struct {
	v      *T
	apiErr *models.ApiError
}

func igWrap[T any](v *T, apiErr *models.ApiError) igR[T] { return igR[T]{v, apiErr} }

func (r igR[T]) ok(t *testing.T) *T {
	t.Helper()
	return igOK(t, r.v, r.apiErr)
}

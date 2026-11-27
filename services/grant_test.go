package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/pkg/account"
	"9itchi-payments-api/repositories"
	"9itchi-payments-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGiveGrants(t *testing.T) {
	ctx := context.Background()
	var gotEmails []string
	var gotNote, gotAdmin string
	repo := &mocks.FreeGrantRepository{UpsertFn: func(_ context.Context, admin string, emails []string, q map[string]int, note string) ([]db.FreeGrant, error) {
		gotAdmin, gotEmails, gotNote = admin, emails, note
		return []db.FreeGrant{{Email: emails[0], Quotas: q}}, nil
	}}
	svc := NewFreeGrantService(repo)
	quotas := map[string]int{"free_applications": 10}

	t.Run("same quotas for many emails, cleaned and deduped", func(t *testing.T) {
		_, apiErr := svc.GiveGrants(ctx, "admin", api.GiveFreeGrantsRequest{
			Emails: []string{" Ali@ENIS.tn ", "ali@enis.tn", "", "sara@gmail.com"}, Quotas: quotas, Note: "  pilot  "})
		require.Nil(t, apiErr)
		assert.Equal(t, []string{"ali@enis.tn", "sara@gmail.com"}, gotEmails)
		assert.Equal(t, "pilot", gotNote)
		assert.Equal(t, "admin", gotAdmin)
	})
	bad := []api.GiveFreeGrantsRequest{
		{Emails: []string{"not-an-email"}, Quotas: quotas},
		{Emails: []string{}, Quotas: quotas},
		{Emails: []string{"a@b.tn"}, Quotas: map[string]int{}},
		{Emails: []string{"a@b.tn"}, Quotas: map[string]int{"resumes": -1}},
		{Emails: []string{"a@b.tn"}, Quotas: map[string]int{" ": 1}},
		{Emails: []string{"a@b.tn"}, Quotas: quotas, Note: strings.Repeat("x", 201)},
	}
	for i, req := range bad {
		_, apiErr := svc.GiveGrants(ctx, "admin", req)
		require.NotNil(t, apiErr, "case %d", i)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code, "case %d", i)
	}
	t.Run("too many emails", func(t *testing.T) {
		many := make([]string, 501)
		for i := range many {
			many[i] = strings.Repeat("a", i+1) + "@b.tn"
		}
		_, apiErr := svc.GiveGrants(ctx, "admin", api.GiveFreeGrantsRequest{Emails: many, Quotas: quotas})
		require.NotNil(t, apiErr)
	})
}

func TestUpdateDeleteListGrants(t *testing.T) {
	ctx := context.Background()
	var got string
	repo := &mocks.FreeGrantRepository{
		UpdateFn: func(_ context.Context, _, email string, q map[string]int, note string) (*db.FreeGrant, error) {
			got = email
			if email == "none@b.tn" {
				return nil, repositories.ErrNotFound
			}
			return &db.FreeGrant{Email: email, Quotas: q, Note: note}, nil
		},
		DeleteFn: func(_ context.Context, email string) error {
			got = email
			return repositories.ErrNotFound
		},
	}
	svc := NewFreeGrantService(repo)
	out, apiErr := svc.UpdateGrant(ctx, "admin", " A@B.tn", api.UpdateFreeGrantRequest{Quotas: map[string]int{"resumes": 1}})
	require.Nil(t, apiErr)
	assert.Equal(t, "a@b.tn", out.Email)
	_, apiErr = svc.UpdateGrant(ctx, "admin", "none@b.tn", api.UpdateFreeGrantRequest{Quotas: map[string]int{"resumes": 1}})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)
	_, apiErr = svc.UpdateGrant(ctx, "admin", "a@b.tn", api.UpdateFreeGrantRequest{Quotas: map[string]int{}})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)

	apiErr = svc.DeleteGrant(ctx, "X@B.tn")
	require.NotNil(t, apiErr)
	assert.Equal(t, "x@b.tn", got)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)

	list, apiErr := svc.ListGrants(ctx)
	require.Nil(t, apiErr)
	assert.NotNil(t, list, "an empty list is [] not null")
}

func TestFreeGrantsApply(t *testing.T) {
	ctx := context.Background()
	free := &db.Plan{ID: "f", Name: FreePlanName, Quotas: map[string]int{"resumes": 0, "free_applications": 0, "cv_edit": 0}}
	grant := &db.FreeGrant{Email: "ali@enis.tn", Quotas: map[string]int{"resumes": 1, "free_applications": 5}}
	grants := &mocks.FreeGrantRepository{GetFn: func(_ context.Context, email string) (*db.FreeGrant, error) {
		if email == "ali@enis.tn" {
			return grant, nil
		}
		return nil, repositories.ErrNotFound
	}}
	user := func(email string, verified bool, err error) *mocks.Users {
		return &mocks.Users{UserFn: func(context.Context, string) (*account.User, error) {
			return &account.User{Email: email, EmailVerified: verified}, err
		}}
	}

	g := &freeGrants{repo: grants, users: user("Ali@Enis.tn", true, nil)}
	out, ok := g.apply(ctx, userID, free)
	require.True(t, ok)
	assert.Equal(t, map[string]int{"resumes": 1, "free_applications": 5, "cv_edit": 0}, out.Quotas)
	assert.Equal(t, 0, free.Quotas["resumes"], "the Free plan itself is untouched")

	cases := map[string]*freeGrants{
		"unverified email": {repo: grants, users: user("ali@enis.tn", false, nil)},
		"no grant":         {repo: grants, users: user("sara@gmail.com", true, nil)},
		"account down":     {repo: grants, users: user("", false, errors.New("down"))},
		"grants db down": {repo: &mocks.FreeGrantRepository{GetFn: func(context.Context, string) (*db.FreeGrant, error) {
			return nil, errors.New("db down")
		}}, users: user("ali@enis.tn", true, nil)},
		"no resolver": nil,
	}
	for name, g := range cases {
		out, ok := g.apply(ctx, userID, free)
		assert.False(t, ok, name)
		assert.Same(t, free, out, name)
	}
}

func TestGrantedQuotasAndSubscription(t *testing.T) {
	ctx := context.Background()
	free := &db.Plan{ID: "f", Name: FreePlanName, Quotas: map[string]int{"free_applications": 0}}
	plans := &mocks.PlanRepository{GetByNameFn: func(context.Context, string) (*db.Plan, error) { return free, nil }}
	noSub := &mocks.SubscriptionRepository{
		GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) { return nil, repositories.ErrNotFound },
		LastEndedFn:       func(context.Context, string) (*db.Subscription, error) { return nil, repositories.ErrNotFound },
	}
	grants := &freeGrants{
		repo: &mocks.FreeGrantRepository{GetFn: func(context.Context, string) (*db.FreeGrant, error) {
			return &db.FreeGrant{Quotas: map[string]int{"free_applications": 7}}, nil
		}},
		users: &mocks.Users{UserFn: func(context.Context, string) (*account.User, error) {
			return &account.User{Email: "a@b.tn", EmailVerified: true}, nil
		}},
	}

	svc := NewPlanService(plans, plans, noSub).(*planService)
	svc.grants = grants
	q, apiErr := svc.GetQuotas(ctx, userID)
	require.Nil(t, apiErr)
	assert.Equal(t, 7, q.Quotas["free_applications"])
	assert.Equal(t, FreePlanName, q.PlanName)

	subs := newSubSvc(noSub, plans, nil, nil, nil)
	subs.grants = grants
	me, apiErr := subs.MySubscription(ctx, userID)
	require.Nil(t, apiErr)
	assert.True(t, me.IsFree)
	assert.True(t, me.Granted)
	assert.Equal(t, 7, me.Plan.Quotas["free_applications"])
}

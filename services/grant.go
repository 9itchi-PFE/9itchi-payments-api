package services

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/pkg/account"
	"9itchi-payments-api/repositories"
	"9itchi-payments-api/utils"
)

const (
	maxGrantEmails = 500
	maxGrantNote   = 200
)

var grantEmailRe = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9\-]+(\.[a-z0-9\-]+)+$`)

// FreeGrantService is the free usage the team gives to chosen emails (admin).
type FreeGrantService interface {
	// GET /admin/free-grants
	ListGrants(ctx context.Context) ([]db.FreeGrant, *models.ApiError)
	// POST /admin/free-grants: the same quotas to one or several emails
	GiveGrants(ctx context.Context, adminID string, req api.GiveFreeGrantsRequest) ([]db.FreeGrant, *models.ApiError)
	// PUT /admin/free-grants/:email
	UpdateGrant(ctx context.Context, adminID, email string, req api.UpdateFreeGrantRequest) (*db.FreeGrant, *models.ApiError)
	// DELETE /admin/free-grants/:email
	DeleteGrant(ctx context.Context, email string) *models.ApiError
}

type freeGrantService struct {
	repo repositories.FreeGrantRepository
}

func NewFreeGrantService(repo repositories.FreeGrantRepository) FreeGrantService {
	return &freeGrantService{repo: repo}
}

func (s *freeGrantService) ListGrants(ctx context.Context) ([]db.FreeGrant, *models.ApiError) {
	out, err := s.repo.List(ctx)
	if err != nil {
		return nil, repoError(err, "grant not found")
	}
	if out == nil {
		out = []db.FreeGrant{}
	}
	return out, nil
}

func (s *freeGrantService) GiveGrants(ctx context.Context, adminID string, req api.GiveFreeGrantsRequest) ([]db.FreeGrant, *models.ApiError) {
	emails, apiErr := cleanGrantEmails(req.Emails)
	if apiErr != nil {
		return nil, apiErr
	}
	note, apiErr := checkGrant(req.Quotas, req.Note)
	if apiErr != nil {
		return nil, apiErr
	}
	out, err := s.repo.Upsert(ctx, adminID, emails, req.Quotas, note)
	if err != nil {
		return nil, repoError(err, "grant not found")
	}
	return out, nil
}

func (s *freeGrantService) UpdateGrant(ctx context.Context, adminID, email string, req api.UpdateFreeGrantRequest) (*db.FreeGrant, *models.ApiError) {
	email = normalizeEmail(email)
	note, apiErr := checkGrant(req.Quotas, req.Note)
	if apiErr != nil {
		return nil, apiErr
	}
	out, err := s.repo.Update(ctx, adminID, email, req.Quotas, note)
	if err != nil {
		return nil, repoError(err, "grant not found")
	}
	return out, nil
}

func (s *freeGrantService) DeleteGrant(ctx context.Context, email string) *models.ApiError {
	if err := s.repo.Delete(ctx, normalizeEmail(email)); err != nil {
		return repoError(err, "grant not found")
	}
	return nil
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// cleanGrantEmails lower-cases, trims and dedupes the emails; one malformed email refuses the whole list.
func cleanGrantEmails(in []string) ([]string, *models.ApiError) {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		e := normalizeEmail(raw)
		if e == "" || seen[e] {
			continue
		}
		if !grantEmailRe.MatchString(e) {
			return nil, badRequest("invalid email: " + e)
		}
		seen[e] = true
		out = append(out, e)
	}
	if len(out) == 0 || len(out) > maxGrantEmails {
		return nil, badRequest("emails: give between 1 and 500 emails")
	}
	return out, nil
}

// checkGrant refuses an empty quota set, a negative value or a note too long; it returns the trimmed note.
func checkGrant(quotas map[string]int, note string) (string, *models.ApiError) {
	if len(quotas) == 0 {
		return "", badRequest("quotas: give at least one quota")
	}
	for k, v := range quotas {
		if strings.TrimSpace(k) == "" || v < 0 {
			return "", badRequest("quotas must have non-empty keys and values >= 0")
		}
	}
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > maxGrantNote {
		return "", badRequest("note: 200 characters at most")
	}
	return note, nil
}

// UserDirectory tells who an account is (account-api).
type UserDirectory interface {
	User(ctx context.Context, id string) (*account.User, error)
}

// freeGrants lays the grant of a user's email over the Free plan. A nil *freeGrants gives nothing.
type freeGrants struct {
	repo  repositories.FreeGrantRepository
	users UserDirectory
}

// apply returns the plan with the user's grant laid over it key by key, and whether a grant applied. On the Free
// plan the grant wins; on a paid plan (paid) the more generous of the two wins, so a subscription never loses usage.
// Only an email vouched for by a Google or LinkedIn sign-in counts: anyone could sign up with a password on someone
// else's email. When the grant cannot be read the student gets the plain plan (logged), never an error.
func (g *freeGrants) apply(ctx context.Context, userID string, free *db.Plan, paid bool) (*db.Plan, bool) {
	if g == nil || g.repo == nil || g.users == nil || free == nil {
		return free, false
	}
	u, err := g.users.User(ctx, userID)
	if err != nil {
		utils.Logger.Logger.Errorw("free grant: read user", "error", err)
		return free, false
	}
	if !u.EmailVerified {
		return free, false
	}
	grant, err := g.repo.Get(ctx, normalizeEmail(u.Email))
	if err != nil {
		if !errors.Is(err, repositories.ErrNotFound) {
			utils.Logger.Logger.Errorw("free grant: read grant", "error", err)
		}
		return free, false
	}
	out := *free
	out.Quotas = make(map[string]int, len(free.Quotas)+len(grant.Quotas))
	for k, v := range free.Quotas {
		out.Quotas[k] = v
	}
	for k, v := range grant.Quotas {
		if !paid {
			out.Quotas[k] = v
			continue
		}
		// A paid plan keeps what it gives when it is more: missing or negative = unlimited.
		if cur, ok := out.Quotas[k]; ok && cur >= 0 && (v < 0 || v > cur) {
			out.Quotas[k] = v
		}
	}
	return &out, true
}

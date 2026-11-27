package services

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/repositories"
)

// domainRe is an email domain once lower-cased: labels of letters, digits and hyphens, at least one dot.
var domainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// StudentDiscountService holds the business logic for the student discount.
type StudentDiscountService interface {
	// GET /admin/student-discount: the percentage and the student domains
	GetStudentDiscount(ctx context.Context) (*api.StudentDiscountResponse, *models.ApiError)
	// PUT /admin/student-discount: replace the percentage and the domains
	UpdateStudentDiscount(ctx context.Context, userID string, req api.UpdateStudentDiscountRequest) (*api.StudentDiscountResponse, *models.ApiError)
	// GET /student-discount/me: whether the signed-in user gets it at checkout
	MyStudentDiscount(ctx context.Context, email string, verified bool) (*api.MyStudentDiscountResponse, *models.ApiError)
}

type studentDiscountService struct {
	repo repositories.StudentDiscountRepository
}

func NewStudentDiscountService(repo repositories.StudentDiscountRepository) StudentDiscountService {
	return &studentDiscountService{repo: repo}
}

// studentDomain is the student domain an email is on (the domain itself or a parent of it), "" when none.
func studentDomain(email string, domains []string) string {
	_, host, ok := strings.Cut(strings.ToLower(strings.TrimSpace(email)), "@")
	if !ok || host == "" {
		return ""
	}
	for _, d := range domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return d
		}
	}
	return ""
}

// studentPercent is the percentage a user gets: only a verified email on a student domain, only when it is on.
func studentPercent(d *db.StudentDiscount, email string, verified bool) int {
	if d == nil || d.Percent <= 0 || !verified || studentDomain(email, d.Domains) == "" {
		return 0
	}
	return d.Percent
}

func studentResponse(d *db.StudentDiscount) *api.StudentDiscountResponse {
	domains := d.Domains
	if domains == nil {
		domains = []string{}
	}
	return &api.StudentDiscountResponse{Percent: d.Percent, Domains: domains, UpdatedAt: d.UpdatedAt}
}

func (s *studentDiscountService) GetStudentDiscount(ctx context.Context) (*api.StudentDiscountResponse, *models.ApiError) {
	d, err := s.repo.Get(ctx)
	if err != nil {
		return nil, repoError(err, "student discount not found")
	}
	return studentResponse(d), nil
}

func (s *studentDiscountService) UpdateStudentDiscount(ctx context.Context, userID string, req api.UpdateStudentDiscountRequest) (*api.StudentDiscountResponse, *models.ApiError) {
	if !isUUID(userID) {
		return nil, badRequest("invalid user id")
	}
	if req.Percent == nil || *req.Percent < 0 || *req.Percent > 100 {
		return nil, badRequest("percent must be between 0 and 100")
	}
	domains := make([]string, 0, len(req.Domains))
	for _, raw := range req.Domains {
		// Admins paste "@enis.tn" or ".enis.tn" as often as "enis.tn".
		d := strings.TrimLeft(strings.ToLower(strings.TrimSpace(raw)), "@.")
		if d == "" {
			continue
		}
		if len(d) > 253 || !domainRe.MatchString(d) {
			return nil, badRequest("invalid domain: " + raw)
		}
		if !slices.Contains(domains, d) {
			domains = append(domains, d)
		}
	}
	out, err := s.repo.Update(ctx, userID, &db.StudentDiscount{Percent: *req.Percent, Domains: domains})
	if err != nil {
		return nil, repoError(err, "student discount not found")
	}
	return studentResponse(out), nil
}

func (s *studentDiscountService) MyStudentDiscount(ctx context.Context, email string, verified bool) (*api.MyStudentDiscountResponse, *models.ApiError) {
	d, err := s.repo.Get(ctx)
	if err != nil {
		return nil, repoError(err, "student discount not found")
	}
	domain := ""
	if d.Percent > 0 {
		domain = studentDomain(email, d.Domains)
	}
	if domain == "" {
		return &api.MyStudentDiscountResponse{}, nil
	}
	if !verified {
		return &api.MyStudentDiscountResponse{Percent: d.Percent, Domain: domain, NeedsVerifiedEmail: true}, nil
	}
	return &api.MyStudentDiscountResponse{Eligible: true, Percent: d.Percent, Domain: domain}, nil
}

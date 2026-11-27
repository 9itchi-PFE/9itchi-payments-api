package handlers

import (
	"context"
	"net/http"
	"testing"

	"9itchi-payments-api/models/db"
	"9itchi-payments-api/repositories"
	"9itchi-payments-api/services"
	"9itchi-payments-api/services/mocks"

	"github.com/stretchr/testify/assert"
)

func TestFreeGrantHandlers(t *testing.T) {
	var deleted string
	svc := services.NewFreeGrantService(&mocks.FreeGrantRepository{
		DeleteFn: func(_ context.Context, email string) error {
			deleted = email
			if email == "none@b.tn" {
				return repositories.ErrNotFound
			}
			return nil
		},
		ListFn: func(context.Context) ([]db.FreeGrant, error) { return []db.FreeGrant{{Email: "a@b.tn"}}, nil },
	})

	w := do(http.MethodPost, "/admin/free-grants", "/admin/free-grants", `{"emails":["A@b.tn","c@d.tn"],"quotas":{"free_applications":5}}`, GiveFreeGrants(svc))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"a@b.tn"`)
	w = do(http.MethodPost, "/admin/free-grants", "/admin/free-grants", `{"emails":["x"],"quotas":{"resumes":1}}`, GiveFreeGrants(svc))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	w = do(http.MethodPost, "/admin/free-grants", "/admin/free-grants", `{`, GiveFreeGrants(svc))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = do(http.MethodPut, "/admin/free-grants/:email", "/admin/free-grants/a%40b.tn", `{"quotas":{"resumes":1},"note":"x"}`, UpdateFreeGrant(svc))
	assert.Equal(t, http.StatusOK, w.Code)

	w = do(http.MethodDelete, "/admin/free-grants/:email", "/admin/free-grants/a%40b.tn", "", DeleteFreeGrant(svc))
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "a@b.tn", deleted, "the email is URL-decoded")
	w = do(http.MethodDelete, "/admin/free-grants/:email", "/admin/free-grants/none@b.tn", "", DeleteFreeGrant(svc))
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = do(http.MethodGet, "/admin/free-grants", "/admin/free-grants", "", ListFreeGrants(svc))
	assert.Equal(t, http.StatusOK, w.Code)
}

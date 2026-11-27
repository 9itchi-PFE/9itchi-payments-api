package account

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/internal/users/u1", r.URL.Path)
		assert.Equal(t, "k", r.Header.Get("x-api-key"))
		_, _ = w.Write([]byte(`{"id":"u1","email":"a@b.tn","email_verified":true}`))
	}))
	defer srv.Close()
	u, err := New(srv.URL, "k").User(context.Background(), "u1")
	require.NoError(t, err)
	assert.Equal(t, &User{ID: "u1", Email: "a@b.tn", EmailVerified: true}, u)

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	defer down.Close()
	_, err = New(down.URL, "k").User(context.Background(), "u1")
	assert.Error(t, err)
}

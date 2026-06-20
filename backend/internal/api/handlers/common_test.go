package handlers

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrResponse(t *testing.T) {
	c, rec := newCtx(http.MethodGet, "/", "", nil)
	err := errResponse(c, http.StatusTeapot, "boom")

	assert.NoError(t, err)
	assert.Equal(t, http.StatusTeapot, rec.Code)
	assert.JSONEq(t, `{"error":"boom"}`, rec.Body.String())
}

func TestTenantFromContext_Present(t *testing.T) {
	tenant := testTenant()
	c, _ := newCtx(http.MethodGet, "/", "", tenant)

	got := TenantFromContext(c)
	assert.Equal(t, tenant, got)
}

func TestTenantFromContext_Missing(t *testing.T) {
	c, _ := newCtx(http.MethodGet, "/", "", nil)

	got := TenantFromContext(c)
	assert.Nil(t, got)
}

func TestTenantFromContext_WrongType(t *testing.T) {
	c, _ := newCtx(http.MethodGet, "/", "", nil)
	c.Set("tenant", "not-a-tenant")

	got := TenantFromContext(c)
	assert.Nil(t, got)
}

func TestRequireTenant_Present(t *testing.T) {
	tenant := testTenant()
	c, _ := newCtx(http.MethodGet, "/", "", tenant)

	got, err := RequireTenant(c)
	assert.NoError(t, err)
	assert.Equal(t, tenant, got)
}

func TestRequireTenant_Missing(t *testing.T) {
	c, rec := newCtx(http.MethodGet, "/", "", nil)

	got, err := RequireTenant(c)
	assert.NoError(t, err)
	assert.Nil(t, got)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestPaginationParams_Normalize(t *testing.T) {
	cases := []struct {
		name       string
		in         PaginationParams
		wantPage   int
		wantLimit  int
	}{
		{"defaults zero", PaginationParams{Page: 0, Limit: 0}, 1, 20},
		{"negative page", PaginationParams{Page: -5, Limit: 10}, 1, 10},
		{"negative limit", PaginationParams{Page: 2, Limit: -1}, 2, 20},
		{"limit too high", PaginationParams{Page: 1, Limit: 101}, 1, 20},
		{"limit at max boundary", PaginationParams{Page: 1, Limit: 100}, 1, 100},
		{"valid values untouched", PaginationParams{Page: 3, Limit: 50}, 3, 50},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.in
			p.Normalize()
			assert.Equal(t, tc.wantPage, p.Page)
			assert.Equal(t, tc.wantLimit, p.Limit)
		})
	}
}

func TestPaginationParams_Offset(t *testing.T) {
	cases := []struct {
		name string
		in   PaginationParams
		want int
	}{
		{"page 1", PaginationParams{Page: 1, Limit: 20}, 0},
		{"page 2", PaginationParams{Page: 2, Limit: 20}, 20},
		{"page 3 limit 50", PaginationParams{Page: 3, Limit: 50}, 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.in.Offset())
		})
	}
}

func TestHealthHandler_Check(t *testing.T) {
	h := NewHealthHandler()
	c, rec := newCtx(http.MethodGet, "/health", "", nil)

	err := h.Check(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

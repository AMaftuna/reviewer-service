package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"reviewer-service/internal/config"
	"reviewer-service/internal/db"
	httpx "reviewer-service/internal/http"
	"reviewer-service/internal/repo"
	"reviewer-service/internal/service"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := config.FromEnv()
	require.NotEmpty(t, cfg.DatabaseURL)

	pool, err := db.NewPool(context.Background(), cfg.DatabaseURL)
	require.NoError(t, err)

	rp := repo.New(pool)
	svc := service.New(rp)
	handler := httpx.NewRouter(svc)
	return httptest.NewServer(handler)
}

func TestE2E_CreatePR_AssignsActiveReviewers(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set")
	}
	ts := newTestServer(t)
	defer ts.Close()

	do(t, ts, "POST", "/team/add", map[string]any{
		"team_name": "payments",
		"members": []map[string]any{
			{"user_id": "u1", "username": "Alice", "is_active": true},
			{"user_id": "u2", "username": "Bob", "is_active": true},
			{"user_id": "u3", "username": "Cara", "is_active": true},
		},
	}, 201, nil)

	prReq := map[string]any{
		"pull_request_id":   "pr1",
		"pull_request_name": "Test PR1",
		"author_id":         "u1",
	}
	var prResp struct {
		PR struct {
			Assigned []string `json:"assigned_reviewers"`
			Status   string   `json:"status"`
		} `json:"pr"`
	}
	do(t, ts, "POST", "/pullRequest/create", prReq, 201, &prResp)

	require.Equal(t, "OPEN", prResp.PR.Status)
	require.Len(t, prResp.PR.Assigned, 2)
	require.NotContains(t, prResp.PR.Assigned, "u1")
}

func TestE2E_Deactivate_SafeReassign(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set")
	}
	ts := newTestServer(t)
	defer ts.Close()

	do(t, ts, "POST", "/team/add", map[string]any{
		"team_name": "core",
		"members": []map[string]any{
			{"user_id": "a1", "username": "A1", "is_active": true},
			{"user_id": "a2", "username": "A2", "is_active": true},
			{"user_id": "a3", "username": "A3", "is_active": true},
		},
	}, 201, nil)

	do(t, ts, "POST", "/pullRequest/create", map[string]any{
		"pull_request_id":   "pr2",
		"pull_request_name": "PR2",
		"author_id":         "a1",
	}, 201, nil)

	var resp map[string]any
	do(t, ts, "POST", "/team/deactivate", map[string]any{
		"team_name": "core",
		"user_ids":  []string{"a2"},
	}, 200, &resp)

	var pr map[string]any
	do(t, ts, "POST", "/pullRequest/merge", map[string]any{
		"pull_request_id": "pr2",
	}, 200, &pr)
}

func do(t *testing.T, ts *httptest.Server, method, path string, body any, want int, out any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req, _ := http.NewRequest(method, ts.URL+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, want, res.StatusCode)
	if out != nil {
		require.NoError(t, json.NewDecoder(res.Body).Decode(out))
	}
}

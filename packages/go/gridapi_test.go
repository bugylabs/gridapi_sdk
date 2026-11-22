package gridapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	gridapi "github.com/bugylabs/gridapi-sdk/packages/go"
)

const (
	testAccountKey = "gp_live_testkey"
	testProjectKey = "gapi_live_testproject"
)

// stubAPI implements only the discovery and data paths Verify needs.
type stubAPI struct {
	authenticated bool
	projects      []gridapi.Project

	mu    sync.Mutex
	calls map[string]int
	order []string
}

func (s *stubAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.calls == nil {
		s.calls = make(map[string]int)
	}
	s.calls[r.URL.Path]++
	s.order = append(s.order, r.URL.Path)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/api/v1/me":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"authenticated": s.authenticated,
			"account":       map[string]any{"id": "a1", "name": "Acme"},
		})
	case "/api/v1/me/projects":
		_ = json.NewEncoder(w).Encode(map[string]any{"projects": s.projects})
	case "/api/v1/me/usage":
		_, _ = w.Write([]byte(`{"totalApiCallsUsed":1,"totalApiCallsLeft":99}`))
	case "/api/v1/acme/users":
		_, _ = w.Write([]byte(`{"data":[],"meta":{"total":0,"limit":0,"offset":0}}`))
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Not found"}`))
	}
}

func (s *stubAPI) count(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[path]
}

func (s *stubAPI) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.order...)
}

func (s *stubAPI) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = make(map[string]int)
	s.order = nil
}

func newStub(t *testing.T, authenticated bool, projects ...gridapi.Project) (*httptest.Server, *stubAPI) {
	t.Helper()
	stub := &stubAPI{authenticated: authenticated, projects: projects}
	return httptest.NewServer(stub), stub
}

func newTestClient(t *testing.T, base string, extra ...gridapi.Option) *gridapi.Client {
	t.Helper()
	opts := []gridapi.Option{
		gridapi.WithAPIKey(testAccountKey),
		gridapi.WithProjectKey(testProjectKey),
		gridapi.WithBaseURL(base),
	}
	opts = append(opts, extra...)
	c, err := gridapi.NewClient(opts...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestNewClientRequiresBothKeys(t *testing.T) {
	t.Setenv("GRIDAPI_API_KEY", "")
	t.Setenv("GRIDAPI_PROJECT_KEY", "")

	cases := []struct {
		name string
		opts []gridapi.Option
	}{
		{"missing account key", []gridapi.Option{gridapi.WithProjectKey(testProjectKey)}},
		{"missing project key", []gridapi.Option{gridapi.WithAPIKey(testAccountKey)}},
		{"malformed account prefix", []gridapi.Option{gridapi.WithAPIKey("pk_live_secret"), gridapi.WithProjectKey(testProjectKey)}},
		{"malformed project prefix", []gridapi.Option{gridapi.WithAPIKey(testAccountKey), gridapi.WithProjectKey("gp_live_secret")}},
		{"empty account suffix", []gridapi.Option{gridapi.WithAPIKey("gp_live_"), gridapi.WithProjectKey(testProjectKey)}},
		{"empty project suffix", []gridapi.Option{gridapi.WithAPIKey(testAccountKey), gridapi.WithProjectKey("gapi_live_")}},
		{"no env keys", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := gridapi.NewClient(tc.opts...)
			var cfg *gridapi.ConfigurationError
			if !errors.As(err, &cfg) {
				t.Fatalf("error = %v, want *ConfigurationError", err)
			}
			for _, secret := range []string{"pk_live_secret", "gp_live_secret", testAccountKey, testProjectKey} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("error message leaked a key value: %q", err.Error())
				}
			}
		})
	}

	t.Run("valid keys from env", func(t *testing.T) {
		t.Setenv("GRIDAPI_API_KEY", testAccountKey)
		t.Setenv("GRIDAPI_PROJECT_KEY", testProjectKey)
		if _, err := gridapi.NewClient(); err != nil {
			t.Fatalf("NewClient from env: %v", err)
		}
	})
}

func TestVerify(t *testing.T) {
	projects := []gridapi.Project{
		{ID: "p1", Name: "Acme", Slug: "acme"},
		{ID: "p2", Name: "Beta", Slug: "beta"},
	}
	ctx := context.Background()

	t.Run("successful verify", func(t *testing.T) {
		srv, stub := newStub(t, true, projects...)
		defer srv.Close()
		client := newTestClient(t, srv.URL)
		if err := client.Verify(ctx, "acme"); err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if stub.count("/api/v1/me") != 1 || stub.count("/api/v1/me/projects") != 1 {
			t.Fatalf("discovery calls = %+v", stub.calls)
		}
	})

	t.Run("account and project checks are cached", func(t *testing.T) {
		srv, stub := newStub(t, true, projects...)
		defer srv.Close()
		client := newTestClient(t, srv.URL)
		for _, p := range []string{"acme", "beta", "acme", "p1", ""} {
			if err := client.Verify(ctx, p); err != nil {
				t.Fatalf("Verify(%q): %v", p, err)
			}
		}
		if stub.count("/api/v1/me") != 1 {
			t.Fatalf("/me calls = %d, want 1", stub.count("/api/v1/me"))
		}
		if stub.count("/api/v1/me/projects") != 3 { // acme, beta, p1 (id)
			t.Fatalf("/me/projects calls = %d, want 3", stub.count("/api/v1/me/projects"))
		}
	})

	t.Run("unknown project", func(t *testing.T) {
		srv, _ := newStub(t, true, projects...)
		defer srv.Close()
		client := newTestClient(t, srv.URL)
		err := client.Verify(ctx, "ghost")
		var nf *gridapi.ProjectNotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("error = %v, want *ProjectNotFoundError", err)
		}
		if !strings.Contains(err.Error(), "ghost") {
			t.Fatalf("error missing project name: %v", err)
		}
		if strings.Contains(err.Error(), testAccountKey) || strings.Contains(err.Error(), testProjectKey) {
			t.Fatalf("error leaked a key value: %q", err.Error())
		}
	})

	t.Run("forbidden account", func(t *testing.T) {
		srv, _ := newStub(t, true) // no authorized projects
		defer srv.Close()
		client := newTestClient(t, srv.URL)
		err := client.Verify(ctx, "acme")
		var forbidden *gridapi.AuthorizationError
		if !errors.As(err, &forbidden) {
			t.Fatalf("error = %v, want *AuthorizationError", err)
		}
	})

	t.Run("unauthenticated account", func(t *testing.T) {
		srv, _ := newStub(t, false, projects...)
		defer srv.Close()
		client := newTestClient(t, srv.URL)
		err := client.Verify(ctx, "acme")
		var auth *gridapi.AuthenticationError
		if !errors.As(err, &auth) {
			t.Fatalf("error = %v, want *AuthenticationError", err)
		}
	})

	t.Run("discovery methods do not re-enter Verify", func(t *testing.T) {
		srv, stub := newStub(t, true, projects...)
		defer srv.Close()
		client := newTestClient(t, srv.URL)
		if _, err := client.GetMe(ctx); err != nil {
			t.Fatalf("GetMe: %v", err)
		}
		if _, err := client.ListProjects(ctx); err != nil {
			t.Fatalf("ListProjects: %v", err)
		}
		if _, err := client.ListTables(ctx, "acme"); err != nil {
			t.Fatalf("ListTables: %v", err)
		}
		if _, err := client.GetUsage(ctx); err != nil {
			t.Fatalf("GetUsage: %v", err)
		}
		stub.reset() // Verify below must still perform the account check itself.
		if err := client.Verify(ctx, "acme"); err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if stub.count("/api/v1/me") != 1 || stub.count("/api/v1/me/projects") != 1 {
			t.Fatalf("discovery calls after reset = %+v", stub.calls)
		}
	})

	t.Run("data method verifies before request", func(t *testing.T) {
		srv, stub := newStub(t, true, projects...)
		defer srv.Close()
		client := newTestClient(t, srv.URL)
		if _, err := client.Get(ctx, "acme", "users", nil); err != nil {
			t.Fatalf("Get: %v", err)
		}
		order := stub.requests()
		if len(order) != 3 || order[0] != "/api/v1/me" ||
			order[1] != "/api/v1/me/projects" || order[2] != "/api/v1/acme/users" {
			t.Fatalf("request order = %v", order)
		}
	})

	t.Run("concurrent Verify", func(t *testing.T) {
		srv, stub := newStub(t, true, projects...)
		defer srv.Close()
		client := newTestClient(t, srv.URL)
		var wg sync.WaitGroup
		errs := make(chan error, 40)
		for i := 0; i < 40; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				p := "acme"
				if i%2 == 0 {
					p = "beta"
				}
				if err := client.Verify(ctx, p); err != nil {
					errs <- err
				}
			}(i)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatalf("concurrent Verify: %v", err)
		}
		if stub.count("/api/v1/me") != 1 {
			t.Fatalf("/me calls = %d, want 1", stub.count("/api/v1/me"))
		}
	})
}

var _ gridapi.APIError = (*gridapi.ConfigurationError)(nil)

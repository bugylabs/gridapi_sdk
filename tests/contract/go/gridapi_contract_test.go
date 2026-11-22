// Contract tests for the Go SDK. They start the shared mock server on port
// 8794, then exercise CRUD, filtering, pagination, error mapping, redaction,
// retries, discovery and count. The server process is always killed in
// cleanup.
//
// Run once Go is installed, from the repository root:
//
//	go test ./tests/contract/go -v
package contract_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	gridapi "github.com/bugylabs/gridapi_sdk/packages/go"
)

const (
	mockPort       = "8794"
	mockKey        = "gp_live_testkey"
	mockProjectKey = "gapi_live_testproject"
)

// startMock launches tests/contract/mock-server.mjs and waits for its port.
func startMock(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("node", filepath.Join("..", "mock-server.mjs"), mockPort)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("start mock server: %v", err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})

	addr := net.JoinHostPort("127.0.0.1", mockPort)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return "http://" + addr
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("mock server never became ready: %s", out.String())
	return ""
}

func newClient(t *testing.T, base string, opts ...gridapi.Option) *gridapi.Client {
	t.Helper()
	all := append([]gridapi.Option{
		gridapi.WithAPIKey(mockKey),
		gridapi.WithProjectKey(mockProjectKey),
		gridapi.WithBaseURL(base),
		gridapi.WithTimeout(2 * time.Second),
		gridapi.WithRetryDelay(10 * time.Millisecond),
	}, opts...)
	c, err := gridapi.NewClient(all...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

// countingTransport records how many data-request attempts an operation made.
// Verification's /me and /me/projects discovery calls are not counted.
type countingTransport struct {
	mu   sync.Mutex
	n    int
	base http.RoundTripper
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, "/users") {
		t.mu.Lock()
		t.n++
		t.mu.Unlock()
	}
	return t.base.RoundTrip(req)
}

func (t *countingTransport) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.n
}

func TestContract(t *testing.T) {
	base := startMock(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := newClient(t, base)

	// Subtests run in order: discovery and reads happen before CRUD mutates rows.
	t.Run("discovery", func(t *testing.T) {
		projects, err := client.ListProjects(ctx)
		if err != nil {
			t.Fatalf("ListProjects: %v", err)
		}
		if len(projects) != 2 || projects[0].Slug != "acme" || projects[1].Slug != "beta" {
			t.Fatalf("unexpected projects: %+v", projects)
		}

		sheets, err := client.ListTables(ctx, "acme")
		if err != nil {
			t.Fatalf("ListTables: %v", err)
		}
		if len(sheets) != 2 || sheets[0].Endpoint != "users" || sheets[1].Endpoint != "empty" {
			t.Fatalf("unexpected sheets: %+v", sheets)
		}
		if _, err := client.ListTables(ctx, "does-not-exist"); err == nil {
			t.Fatal("ListTables: expected ProjectNotFoundError")
		} else {
			var notFound *gridapi.ProjectNotFoundError
			if !errors.As(err, &notFound) {
				t.Fatalf("ListTables: want ProjectNotFoundError, got %T", err)
			}
		}

		schema, err := client.GetSchema(ctx, "acme", "USERS")
		if err != nil {
			t.Fatalf("GetSchema: %v", err)
		}
		if schema.Endpoint != "users" || schema.PrimaryKey != "id" || len(schema.Fields) != 2 {
			t.Fatalf("unexpected schema: %+v", schema)
		}
		if _, err := client.GetSchema(ctx, "acme", "nope"); err == nil {
			t.Fatal("GetSchema: expected TableNotFoundError")
		} else {
			var notFound *gridapi.TableNotFoundError
			if !errors.As(err, &notFound) {
				t.Fatalf("GetSchema: want TableNotFoundError, got %T", err)
			}
		}

		me, err := client.GetMe(ctx)
		if err != nil {
			t.Fatalf("GetMe: %v", err)
		}
		if !me.Authenticated || me.Account.Email != "owner@example.com" {
			t.Fatalf("unexpected account: %+v", me)
		}

		usage, err := client.GetUsage(ctx)
		if err != nil {
			t.Fatalf("GetUsage: %v", err)
		}
		if usage.TotalAPICallsUsed != 10 || usage.TotalAPICallsLeft != 900 {
			t.Fatalf("unexpected usage: %+v", usage)
		}
	})

	t.Run("filterAndPagination", func(t *testing.T) {
		all, err := client.Get(ctx, "acme", "users", nil)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if len(all.Data) != 3 || all.Meta.Total != 3 || all.Meta.Limit != 100 || all.Meta.Offset != 0 {
			t.Fatalf("unexpected list: %+v", all)
		}

		active, err := client.Query(ctx, "acme", "users", &gridapi.QueryOptions{Filter: "status:active"})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(active.Data) != 2 || active.Meta.Total != 2 {
			t.Fatalf("unexpected filter result: %+v", active)
		}

		growth, err := client.Query(ctx, "acme", "users", &gridapi.QueryOptions{
			Filter: map[string]string{"team": "growth"},
		})
		if err != nil {
			t.Fatalf("Query(map filter): %v", err)
		}
		if len(growth.Data) != 2 {
			t.Fatalf("unexpected map-filter result: %+v", growth)
		}

		paged, err := client.Query(ctx, "acme", "users", &gridapi.QueryOptions{
			Filter: "status:active", Limit: 1, Page: 2,
		})
		if err != nil {
			t.Fatalf("Query(page): %v", err)
		}
		if len(paged.Data) != 1 || paged.Data[0]["id"] != "2" ||
			paged.Meta.Total != 2 || paged.Meta.Offset != 1 || paged.Meta.Limit != 1 {
			t.Fatalf("unexpected page: %+v", paged)
		}
	})

	t.Run("count", func(t *testing.T) {
		n, err := client.Count(ctx, "acme", "users", "status:active")
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if n != 2 {
			t.Fatalf("Count = %d, want 2", n)
		}
		n, err = client.Count(ctx, "acme", "users", map[string]string{"team": "growth"})
		if err != nil {
			t.Fatalf("Count(map): %v", err)
		}
		if n != 2 {
			t.Fatalf("Count(map) = %d, want 2", n)
		}
		n, err = client.Count(ctx, "acme", "empty", nil)
		if err != nil {
			t.Fatalf("Count(empty): %v", err)
		}
		if n != 0 {
			t.Fatalf("Count(empty) = %d, want 0", n)
		}
	})

	t.Run("crud", func(t *testing.T) {
		created, err := client.Post(ctx, "acme", "users", gridapi.Record{
			"name": "Zed", "status": "active", "team": "ops",
		})
		if err != nil {
			t.Fatalf("Post: %v", err)
		}
		if created.Inserted != 1 || len(created.Records) != 1 {
			t.Fatalf("unexpected create response: %+v", created)
		}
		id, ok := created.Records[0]["id"].(string)
		if !ok || id == "" {
			t.Fatalf("missing created id: %+v", created.Records[0])
		}

		one, err := client.GetByID(ctx, "acme", "users", id)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if (*one)["name"] != "Zed" {
			t.Fatalf("unexpected record: %+v", *one)
		}

		patched, err := client.Patch(ctx, "acme", "users", id, gridapi.Record{"name": "Zed2"})
		if err != nil {
			t.Fatalf("Patch: %v", err)
		}
		if patched.Updated != 1 || patched.Record["name"] != "Zed2" {
			t.Fatalf("unexpected patch response: %+v", patched)
		}

		replaced, err := client.Put(ctx, "acme", "users", id, gridapi.Record{"name": "Zed3"})
		if err != nil {
			t.Fatalf("Put: %v", err)
		}
		if replaced.Updated != 1 || replaced.Record["name"] != "Zed3" {
			t.Fatalf("unexpected put response: %+v", replaced)
		}

		found, err := client.Query(ctx, "acme", "users", &gridapi.QueryOptions{
			Filter: map[string]string{"name": "Zed3"},
		})
		if err != nil {
			t.Fatalf("Query after put: %v", err)
		}
		if len(found.Data) != 1 {
			t.Fatalf("record not found after put: %+v", found.Data)
		}

		deleted, err := client.Delete(ctx, "acme", "users", id)
		if err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if deleted.Deleted != 1 || deleted.ID != id {
			t.Fatalf("unexpected delete response: %+v", deleted)
		}
		if _, err := client.GetByID(ctx, "acme", "users", id); err == nil {
			t.Fatal("GetByID after delete: expected error")
		} else {
			var missing *gridapi.RecordNotFoundError
			if !errors.As(err, &missing) {
				t.Fatalf("want RecordNotFoundError, got %T", err)
			}
		}
	})

	t.Run("errorMapping", func(t *testing.T) {
		t.Setenv("GRIDAPI_API_KEY", "")
		t.Setenv("GRIDAPI_PROJECT_KEY", "")
		if _, err := gridapi.NewClient(); err == nil {
			t.Fatal("NewClient: expected ConfigurationError for missing keys")
		} else {
			var cfg *gridapi.ConfigurationError
			if !errors.As(err, &cfg) {
				t.Fatalf("want ConfigurationError, got %T", err)
			}
		}

		bad := newClient(t, base, gridapi.WithAPIKey("gp_live_bogus"))
		if _, err := bad.Get(ctx, "acme", "users", nil); err == nil {
			t.Fatal("expected 401")
		} else {
			var auth *gridapi.AuthenticationError
			if !errors.As(err, &auth) || auth.Status != 401 {
				t.Fatalf("want AuthenticationError(401), got %T: %v", err, err)
			}
		}

		other := newClient(t, base, gridapi.WithAPIKey("gp_live_otheraccount"))
		if _, err := other.Get(ctx, "acme", "users", nil); err == nil {
			t.Fatal("expected authorization failure")
		} else {
			var forbidden *gridapi.AuthorizationError
			if !errors.As(err, &forbidden) {
				t.Fatalf("want AuthorizationError, got %T: %v", err, err)
			}
		}

		if _, err := client.Get(ctx, "missing", "users", nil); err == nil {
			t.Fatal("expected project not found")
		} else {
			var notFound *gridapi.ProjectNotFoundError
			if !errors.As(err, &notFound) {
				t.Fatalf("want ProjectNotFoundError, got %T: %v", err, err)
			}
		}
		if _, err := client.Get(ctx, "acme", "nope", nil); err == nil {
			t.Fatal("expected 404 table")
		} else {
			var notFound *gridapi.TableNotFoundError
			if !errors.As(err, &notFound) || notFound.Code != "SHEET_NOT_FOUND" {
				t.Fatalf("want TableNotFoundError, got %T: %v", err, err)
			}
		}
		if _, err := client.Patch(ctx, "acme", "users", "nope", gridapi.Record{"name": "x"}); err == nil {
			t.Fatal("expected 404 record")
		} else {
			var notFound *gridapi.RecordNotFoundError
			if !errors.As(err, &notFound) || notFound.Code != "NOT_FOUND" {
				t.Fatalf("want RecordNotFoundError, got %T: %v", err, err)
			}
		}
		if _, err := client.Post(ctx, "acme", "users", gridapi.Record{"__bad": "x"}); err == nil {
			t.Fatal("expected 400 validation")
		} else {
			var invalid *gridapi.ValidationError
			if !errors.As(err, &invalid) || invalid.Status != 400 {
				t.Fatalf("want ValidationError(400), got %T: %v", err, err)
			}
		}

		quotaClient := newClient(t, base+"/fault/quota", gridapi.WithMaxRetries(3))
		if _, err := quotaClient.Get(ctx, "acme", "users", nil); err == nil {
			t.Fatal("expected 429 quota")
		} else {
			var quota *gridapi.QuotaExceededError
			if !errors.As(err, &quota) || quota.Status != 429 {
				t.Fatalf("want QuotaExceededError(429), got %T: %v", err, err)
			}
			var api gridapi.APIError
			if !errors.As(err, &api) || api.IsRetryable() {
				t.Fatalf("quota must be terminal, got %v", api)
			}
		}
	})

	t.Run("redaction", func(t *testing.T) {
		echo := newClient(t, base+"/fault/echo")
		_, err := echo.Get(ctx, "acme", "users", nil)
		if err == nil {
			t.Fatal("expected 400 echo error")
		}
		if strings.Contains(err.Error(), mockKey) {
			t.Fatalf("key leaked into error: %v", err)
		}
		if !strings.Contains(err.Error(), "[REDACTED]") {
			t.Fatalf("error was not redacted: %v", err)
		}
		var invalid *gridapi.ValidationError
		if !errors.As(err, &invalid) {
			t.Fatalf("want ValidationError, got %T", err)
		}
		if strings.Contains(invalid.Message, mockKey) || strings.Contains(invalid.Body, mockKey) {
			t.Fatalf("key leaked into error fields: %+v", invalid)
		}
	})

	t.Run("retries", func(t *testing.T) {
		rateClient := newClient(t, base+"/fault/rate")
		if _, err := rateClient.Get(ctx, "acme", "users", nil); err != nil {
			t.Fatalf("transient 429 was not retried: %v", err)
		}

		transport := &countingTransport{base: http.DefaultTransport}
		always := newClient(t, base+"/fault/always-rate",
			gridapi.WithMaxRetries(2),
			gridapi.WithHTTPClient(&http.Client{Transport: transport}),
		)
		if _, err := always.Get(ctx, "acme", "users", nil); err == nil {
			t.Fatal("expected persistent 429")
		} else {
			var rate *gridapi.RateLimitError
			if !errors.As(err, &rate) {
				t.Fatalf("want RateLimitError, got %T", err)
			}
			var api gridapi.APIError
			if !errors.As(err, &api) || !api.IsRetryable() {
				t.Fatalf("rate limit must be retryable, got %v", api)
			}
		}
		if got := transport.count(); got != 3 {
			t.Fatalf("GET attempts = %d, want 3 (1 + 2 retries)", got)
		}

		// POST must never be retried, even on a transient 429.
		transport = &countingTransport{base: http.DefaultTransport}
		always = newClient(t, base+"/fault/always-rate",
			gridapi.WithMaxRetries(2),
			gridapi.WithHTTPClient(&http.Client{Transport: transport}),
		)
		if _, err := always.Post(ctx, "acme", "users", gridapi.Record{"name": "no-retry"}); err == nil {
			t.Fatal("expected 429 for POST")
		}
		if got := transport.count(); got != 1 {
			t.Fatalf("POST attempts = %d, want 1", got)
		}

		// PATCH is opt-in: off by default, on with WithRetryWrites.
		transport = &countingTransport{base: http.DefaultTransport}
		always = newClient(t, base+"/fault/always-rate",
			gridapi.WithMaxRetries(2),
			gridapi.WithHTTPClient(&http.Client{Transport: transport}),
		)
		_, _ = always.Patch(ctx, "acme", "users", "1", gridapi.Record{"name": "x"})
		if got := transport.count(); got != 1 {
			t.Fatalf("PATCH attempts = %d, want 1 by default", got)
		}
		transport = &countingTransport{base: http.DefaultTransport}
		retryWrites := newClient(t, base+"/fault/always-rate",
			gridapi.WithMaxRetries(2),
			gridapi.WithRetryWrites(true),
			gridapi.WithHTTPClient(&http.Client{Transport: transport}),
		)
		_, _ = retryWrites.Patch(ctx, "acme", "users", "1", gridapi.Record{"name": "x"})
		if got := transport.count(); got != 3 {
			t.Fatalf("PATCH attempts with retryWrites = %d, want 3", got)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		timeout := newClient(t, base+"/fault/timeout",
			gridapi.WithTimeout(50*time.Millisecond),
			gridapi.WithMaxRetries(0),
		)
		_, err := timeout.Get(ctx, "acme", "users", nil)
		if err == nil {
			t.Fatal("expected timeout")
		}
		var timedOut *gridapi.TimeoutError
		if !errors.As(err, &timedOut) {
			t.Fatalf("want TimeoutError, got %T: %v", err, err)
		}
	})

	t.Run("both keys required", func(t *testing.T) {
		t.Setenv("GRIDAPI_API_KEY", "")
		t.Setenv("GRIDAPI_PROJECT_KEY", "")
		cases := []struct {
			name  string
			opts  []gridapi.Option
			leaks string
		}{
			{"missing account key", []gridapi.Option{gridapi.WithProjectKey(mockProjectKey)}, ""},
			{"missing project key", []gridapi.Option{gridapi.WithAPIKey(mockKey)}, ""},
			{"malformed account key", []gridapi.Option{gridapi.WithAPIKey("gapi_live_secret"), gridapi.WithProjectKey(mockProjectKey)}, "gapi_live_secret"},
			{"malformed project key", []gridapi.Option{gridapi.WithAPIKey(mockKey), gridapi.WithProjectKey("gp_live_secret")}, "gp_live_secret"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := gridapi.NewClient(tc.opts...)
				var cfg *gridapi.ConfigurationError
				if !errors.As(err, &cfg) {
					t.Fatalf("want ConfigurationError, got %T: %v", err, err)
				}
				if tc.leaks != "" && strings.Contains(err.Error(), tc.leaks) {
					t.Fatalf("error leaked a key value: %q", err.Error())
				}
			})
		}
	})

	t.Run("verify", func(t *testing.T) {
		if err := client.Verify(ctx, "acme"); err != nil {
			t.Fatalf("Verify(acme): %v", err)
		}
		if err := client.Verify(ctx, "ghost"); err == nil {
			t.Fatal("expected unknown project")
		} else {
			var nf *gridapi.ProjectNotFoundError
			if !errors.As(err, &nf) {
				t.Fatalf("want ProjectNotFoundError, got %T: %v", err, err)
			}
			if strings.Contains(err.Error(), mockKey) || strings.Contains(err.Error(), mockProjectKey) {
				t.Fatalf("error leaked a key value: %q", err.Error())
			}
		}
		other := newClient(t, base, gridapi.WithAPIKey("gp_live_otheraccount"))
		if err := other.Verify(ctx, "acme"); err == nil {
			t.Fatal("expected authorization failure")
		} else {
			var forbidden *gridapi.AuthorizationError
			if !errors.As(err, &forbidden) {
				t.Fatalf("want AuthorizationError, got %T: %v", err, err)
			}
		}
	})
}

// Compile-time guard: the typed errors must satisfy the public APIError
// interface, otherwise errors.As(err, &api) would never match them.
var (
	_ gridapi.APIError = (*gridapi.ConfigurationError)(nil)
	_ gridapi.APIError = (*gridapi.AuthenticationError)(nil)
	_ gridapi.APIError = (*gridapi.QuotaExceededError)(nil)
	_ gridapi.APIError = (*gridapi.NetworkError)(nil)
	_ gridapi.APIError = (*gridapi.TimeoutError)(nil)
)

func ExampleClient_Get() {
	client, err := gridapi.NewClient(gridapi.WithAPIKey("gp_live_example"), gridapi.WithProjectKey("gapi_live_example"))
	if err != nil {
		fmt.Println(err)
		return
	}
	res, err := client.Get(context.Background(), "acme", "users", &gridapi.QueryOptions{Limit: 1})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(len(res.Data))
}

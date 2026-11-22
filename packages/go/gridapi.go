// Package gridapi is the official, stdlib-only Go client for the GridAPI
// public REST API (contract: openapi/openapi.yaml in the gridapi_sdk monorepo).
//
// Two keys are required: GRIDAPI_API_KEY (gp_live_, the account key sent as
// Bearer) and GRIDAPI_PROJECT_KEY (gapi_live_, a local-only scope guard that is
// never transmitted). Every data method verifies the account once and the
// project slug/id/name against the account's authorized projects first. This
// SDK is server-side only: never ship either key in client-side code.
//
// Errors are typed and branchable with errors.As:
//
//	var auth *gridapi.AuthenticationError
//	if errors.As(err, &auth) {
//		// auth.Status, auth.Code, auth.Retryable are promoted from *gridapi.Error.
//	}
//
// For category-agnostic access to Status/Code/Retryable, match the APIError
// interface instead. Every typed error embeds *Error, so both work.
package gridapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBaseURL is the production API root.
	DefaultBaseURL = "https://api.gridapi.in"
	// DefaultTimeout is the per-attempt request timeout.
	DefaultTimeout = 30 * time.Second

	// Path templates intentionally keep the OpenAPI {placeholder} spelling so
	// scripts/validate_contract.py can verify them against the contract.
	pathRecords     = "/api/v1/{projectSlug}/{endpoint}"
	pathRecord      = "/api/v1/{projectSlug}/{endpoint}/{id}"
	pathMe          = "/api/v1/me"
	pathMeProjects  = "/api/v1/me/projects"
	pathMeUsage     = "/api/v1/me/usage"
	pathProjectDocs = "/api/v1/me/projects/{projectRef}/docs"
)

// Record is a single row. Values are normally strings; JSON null decodes to
// nil (the contract's RecordValue is string | null).
type Record map[string]any

// ListMeta is the metadata accompanying a list response.
type ListMeta struct {
	Total    int  `json:"total"`
	Limit    int  `json:"limit"`
	Offset   int  `json:"offset"`
	CacheHit bool `json:"cacheHit,omitempty"`
}

// ListResponse is the shape returned by list/query operations.
type ListResponse[T any] struct {
	Data []T      `json:"data"`
	Meta ListMeta `json:"meta"`
}

// CreateResponse is returned by Post.
type CreateResponse[T any] struct {
	Inserted int `json:"inserted"`
	Records  []T `json:"records"`
}

// UpdateResponse is returned by Patch and Put.
type UpdateResponse[T any] struct {
	Updated int `json:"updated"`
	Record  T   `json:"record"`
}

// DeleteResponse is returned by Delete.
type DeleteResponse struct {
	Deleted int    `json:"deleted"`
	ID      string `json:"id"`
}

// Sheet describes one addressable sheet/endpoint of a project.
type Sheet struct {
	Endpoint       string   `json:"endpoint"`
	Name           string   `json:"name,omitempty"`
	SheetName      string   `json:"sheetName,omitempty"`
	PrimaryKey     string   `json:"primaryKey,omitempty"`
	AllowedMethods []string `json:"allowedMethods,omitempty"`
}

// Project describes an account-authorized project and its sheets.
type Project struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Slug          string  `json:"slug"`
	Role          string  `json:"role"`
	SpreadsheetID string  `json:"spreadsheetId,omitempty"`
	Sheets        []Sheet `json:"sheets"`
}

// TableField is one column of a TableSchema.
type TableField struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// TableSchema is one endpoint entry from a project's /docs response.
type TableSchema struct {
	Endpoint   string            `json:"endpoint"`
	Method     string            `json:"method,omitempty"`
	URL        string            `json:"url,omitempty"`
	PrimaryKey string            `json:"primaryKey,omitempty"`
	Fields     []TableField      `json:"fields,omitempty"`
	Samples    map[string]string `json:"samples,omitempty"`
}

// AccountIdentity is the account object inside an authenticated /me response.
type AccountIdentity struct {
	ID    string `json:"id,omitempty"`
	Email string `json:"email,omitempty"`
	Scope string `json:"scope,omitempty"`
}

// Account is the response from GetMe.
type Account struct {
	Authenticated bool            `json:"authenticated"`
	Account       AccountIdentity `json:"account"`
	Subscription  map[string]any  `json:"subscription,omitempty"`
}

// Usage is the response from GetUsage.
type Usage struct {
	TotalAPICallsUsed int `json:"totalApiCallsUsed"`
	TotalAPICallsLeft int `json:"totalApiCallsLeft"`
}

// QueryOptions holds exact-match filters and pagination. Filter accepts either
// a "col:value,col2:value2" string or a map[string]string. Page is 1-based and
// only takes effect when Limit is also set.
type QueryOptions struct {
	Filter any `json:"-"`
	Page   int `json:"-"`
	Limit  int `json:"-"`
}

// Error is the base error returned by every failing SDK operation. Its fields
// are already redacted, so it is safe to log or print.
// It is an alias for baseError; the unexported name avoids a field/method
// collision with Error() when embedded in the typed errors below.
type Error = baseError

type baseError struct {
	Status    int
	Code      string
	Message   string
	Body      string
	Retryable bool
}

func (e *baseError) Error() string {
	var b strings.Builder
	b.WriteString("GridAPI")
	if e.Status > 0 {
		b.WriteByte(' ')
		b.WriteString(strconv.Itoa(e.Status))
	}
	if e.Code != "" {
		b.WriteString(" [")
		b.WriteString(e.Code)
		b.WriteByte(']')
	}
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	} else if e.Body != "" {
		b.WriteString(": ")
		b.WriteString(e.Body)
	}
	return b.String()
}

// StatusCode, ErrorCode and IsRetryable let APIError be matched with errors.As.
func (e *baseError) StatusCode() int   { return e.Status }
func (e *baseError) ErrorCode() string { return e.Code }
func (e *baseError) IsRetryable() bool { return e.Retryable }

// APIError is implemented by every typed error in this package. Match it with
// errors.As when the specific category is not needed:
//
//	var api gridapi.APIError
//	if errors.As(err, &api) { fmt.Println(api.StatusCode(), api.IsRetryable()) }
type APIError interface {
	error
	StatusCode() int
	ErrorCode() string
	IsRetryable() bool
}

// The typed errors below mirror the JavaScript SDK taxonomy. Match one with
// errors.As; Status, Code, Message, Body and Retryable are promoted from the
// embedded *Error.
type (
	ConfigurationError   struct{ *baseError }
	AuthenticationError  struct{ *baseError }
	AuthorizationError   struct{ *baseError }
	ProjectNotFoundError struct{ *baseError }
	TableNotFoundError   struct{ *baseError }
	RecordNotFoundError  struct{ *baseError }
	ValidationError      struct{ *baseError }
	RateLimitError       struct{ *baseError }
	QuotaExceededError   struct{ *baseError }
	UpstreamError        struct{ *baseError }
	NetworkError         struct{ *baseError }
	TimeoutError         struct{ *baseError }
)

// Config holds client configuration. It is populated from environment
// variables and then overridden by any Option passed to NewClient.
type Config struct {
	APIKey      string
	ProjectKey  string
	BaseURL     string
	Timeout     time.Duration
	MaxRetries  int
	RetryDelay  time.Duration
	RetryWrites bool
	HTTPClient  *http.Client
}

// Option customizes a Client.
type Option func(*Config)

// WithAPIKey overrides GRIDAPI_API_KEY (the account key sent as Bearer).
func WithAPIKey(key string) Option { return func(c *Config) { c.APIKey = key } }

// WithProjectKey overrides GRIDAPI_PROJECT_KEY. It is a local scope guard only:
// the public REST API has no project-key header, so this value is never sent.
func WithProjectKey(key string) Option { return func(c *Config) { c.ProjectKey = key } }

// WithBaseURL overrides GRIDAPI_BASE_URL.
func WithBaseURL(base string) Option { return func(c *Config) { c.BaseURL = base } }

// WithTimeout overrides GRIDAPI_TIMEOUT_MS (per attempt).
func WithTimeout(d time.Duration) Option { return func(c *Config) { c.Timeout = d } }

// WithMaxRetries sets the number of retries after the first attempt. Default 2.
func WithMaxRetries(n int) Option { return func(c *Config) { c.MaxRetries = n } }

// WithRetryDelay sets the exponential backoff base. Default 500ms.
func WithRetryDelay(d time.Duration) Option { return func(c *Config) { c.RetryDelay = d } }

// WithRetryWrites allows retrying PATCH. POST is never retried; GET, DELETE and
// PUT are always retryable.
func WithRetryWrites(enabled bool) Option { return func(c *Config) { c.RetryWrites = enabled } }

// WithHTTPClient supplies a custom *http.Client (useful for proxies or tests).
func WithHTTPClient(h *http.Client) Option { return func(c *Config) { c.HTTPClient = h } }

// Client is a GridAPI client. It is safe for concurrent use.
type Client struct {
	apiKey      string
	projectKey  string
	baseURL     string
	timeout     time.Duration
	maxRetries  int
	retryDelay  time.Duration
	retryWrites bool
	httpClient  *http.Client

	verifyOnce sync.Once
	verifyErr  error
	projectsMu sync.Mutex
	verified   map[string]struct{}
}

// NewClient reads GRIDAPI_API_KEY, GRIDAPI_PROJECT_KEY, GRIDAPI_BASE_URL and
// GRIDAPI_TIMEOUT_MS, then applies opts. Both keys are required and shaped like
// gp_live_... / gapi_live_...; an absent or malformed key returns a
// ConfigurationError. Key values never appear in error messages.
func NewClient(opts ...Option) (*Client, error) {
	cfg := Config{
		APIKey:     os.Getenv("GRIDAPI_API_KEY"),
		ProjectKey: os.Getenv("GRIDAPI_PROJECT_KEY"),
		BaseURL:    os.Getenv("GRIDAPI_BASE_URL"),
		Timeout:    DefaultTimeout,
		MaxRetries: 2,
		RetryDelay: 500 * time.Millisecond,
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if raw := os.Getenv("GRIDAPI_TIMEOUT_MS"); raw != "" {
		if ms, err := strconv.Atoi(raw); err == nil && ms > 0 {
			cfg.Timeout = time.Duration(ms) * time.Millisecond
		}
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if err := requiredKey("GRIDAPI_API_KEY", cfg.APIKey, "gp_live_", "WithAPIKey"); err != nil {
		return nil, err
	}
	if err := requiredKey("GRIDAPI_PROJECT_KEY", cfg.ProjectKey, "gapi_live_", "WithProjectKey"); err != nil {
		return nil, err
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 500 * time.Millisecond
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{}
	}
	return &Client{
		apiKey:      cfg.APIKey,
		projectKey:  cfg.ProjectKey,
		baseURL:     base,
		timeout:     cfg.Timeout,
		maxRetries:  cfg.MaxRetries,
		retryDelay:  cfg.RetryDelay,
		retryWrites: cfg.RetryWrites,
		httpClient:  cfg.HTTPClient,
		verified:    make(map[string]struct{}),
	}, nil
}

// requiredKey validates a credential's presence and shape without ever
// echoing the value. env/option are names only, never secrets.
func requiredKey(env, key, prefix, option string) error {
	if strings.TrimSpace(key) == "" {
		return &ConfigurationError{&Error{
			Message: "GridAPI: missing key. Set " + env + " or use " + option + ".",
		}}
	}
	if !strings.HasPrefix(key, prefix) || len(key) == len(prefix) {
		return &ConfigurationError{&Error{
			Message: "GridAPI: " + env + " must be a " + prefix + "... key.",
		}}
	}
	return nil
}

// Verify checks the account once, then checks that project (a slug, id or name)
// is one of the account's authorized projects. The second check is cached per
// project. Discovery methods call do directly, so they never re-enter Verify.
func (c *Client) Verify(ctx context.Context, project string) error {
	c.verifyOnce.Do(func() {
		me, err := c.GetMe(ctx)
		if err != nil {
			c.verifyErr = err
			return
		}
		if !me.Authenticated {
			c.verifyErr = &AuthenticationError{&Error{
				Message: "GridAPI: account key is not authenticated.",
			}}
		}
	})
	if c.verifyErr != nil {
		return c.verifyErr
	}
	if project == "" {
		return nil
	}
	c.projectsMu.Lock()
	_, ok := c.verified[project]
	c.projectsMu.Unlock()
	if ok {
		return nil
	}

	projects, err := c.ListProjects(ctx)
	if err != nil {
		return err
	}
	allowed := false
	for _, p := range projects {
		if p.Slug == project || p.ID == project || p.Name == project {
			allowed = true
			break
		}
	}
	if !allowed {
		if len(projects) == 0 {
			return &AuthorizationError{&Error{
				Message: "GridAPI: no projects are authorized for this account and project key.",
			}}
		}
		return &ProjectNotFoundError{&Error{Message: "Project not found: " + project}}
	}
	c.projectsMu.Lock()
	c.verified[project] = struct{}{}
	c.projectsMu.Unlock()
	return nil
}

// Get lists records in a table, optionally filtered and paginated.
func (c *Client) Get(ctx context.Context, project, table string, params *QueryOptions) (*ListResponse[Record], error) {
	if err := c.Verify(ctx, project); err != nil {
		return nil, err
	}
	raw, err := c.do(ctx, http.MethodGet, dataPath(project, table)+buildQuery(params), nil)
	if err != nil {
		return nil, err
	}
	return decodeJSON[ListResponse[Record]](raw)
}

// Query is an alias for Get with the same filter/page/limit options.
func (c *Client) Query(ctx context.Context, project, table string, params *QueryOptions) (*ListResponse[Record], error) {
	return c.Get(ctx, project, table, params)
}

// GetByID retrieves one record by its primary key.
func (c *Client) GetByID(ctx context.Context, project, table, id string) (*Record, error) {
	if err := c.Verify(ctx, project); err != nil {
		return nil, err
	}
	raw, err := c.do(ctx, http.MethodGet, recordPath(project, table, id), nil)
	if err != nil {
		return nil, err
	}
	return decodeJSON[Record](raw)
}

// Post inserts one record (pass a Record) or several (pass a []Record).
// POST is never retried.
func (c *Client) Post(ctx context.Context, project, table string, records any) (*CreateResponse[Record], error) {
	if err := c.Verify(ctx, project); err != nil {
		return nil, err
	}
	raw, err := c.do(ctx, http.MethodPost, dataPath(project, table), records)
	if err != nil {
		return nil, err
	}
	return decodeJSON[CreateResponse[Record]](raw)
}

// Patch partially updates one record. It is retried only when WithRetryWrites
// is enabled.
func (c *Client) Patch(ctx context.Context, project, table, id string, changes Record) (*UpdateResponse[Record], error) {
	if err := c.Verify(ctx, project); err != nil {
		return nil, err
	}
	raw, err := c.do(ctx, http.MethodPatch, recordPath(project, table, id), changes)
	if err != nil {
		return nil, err
	}
	return decodeJSON[UpdateResponse[Record]](raw)
}

// Put replaces one record. It is always retried on transient failures.
func (c *Client) Put(ctx context.Context, project, table, id string, record Record) (*UpdateResponse[Record], error) {
	if err := c.Verify(ctx, project); err != nil {
		return nil, err
	}
	raw, err := c.do(ctx, http.MethodPut, recordPath(project, table, id), record)
	if err != nil {
		return nil, err
	}
	return decodeJSON[UpdateResponse[Record]](raw)
}

// Delete removes one record by its primary key.
func (c *Client) Delete(ctx context.Context, project, table, id string) (*DeleteResponse, error) {
	if err := c.Verify(ctx, project); err != nil {
		return nil, err
	}
	raw, err := c.do(ctx, http.MethodDelete, recordPath(project, table, id), nil)
	if err != nil {
		return nil, err
	}
	return decodeJSON[DeleteResponse](raw)
}

// Count returns the number of records matching filter. It is implemented as a
// limit=1 query and reads meta.total, so no rows are fetched unnecessarily.
func (c *Client) Count(ctx context.Context, project, table string, filter any) (int, error) {
	res, err := c.Get(ctx, project, table, &QueryOptions{Filter: filter, Limit: 1})
	if err != nil {
		return 0, err
	}
	return res.Meta.Total, nil
}

// ListProjects returns every project authorized for the account key.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	raw, err := c.do(ctx, http.MethodGet, pathMeProjects, nil)
	if err != nil {
		return nil, err
	}
	out, err := decodeJSON[projectsResponse](raw)
	if err != nil {
		return nil, err
	}
	return out.Projects, nil
}

// ListTables returns the sheets of a project, matched by slug, id or name.
func (c *Client) ListTables(ctx context.Context, project string) ([]Sheet, error) {
	projects, err := c.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.Slug == project || p.ID == project || p.Name == project {
			return p.Sheets, nil
		}
	}
	return nil, &ProjectNotFoundError{&Error{Message: "GridAPI: project not found: " + project}}
}

// GetSchema returns the schema of one table from the project's docs endpoint.
func (c *Client) GetSchema(ctx context.Context, project, table string) (*TableSchema, error) {
	raw, err := c.do(ctx, http.MethodGet, docsPath(project), nil)
	if err != nil {
		return nil, err
	}
	out, err := decodeJSON[docsResponse](raw)
	if err != nil {
		return nil, err
	}
	for i := range out.Endpoints {
		if strings.EqualFold(out.Endpoints[i].Endpoint, table) {
			return &out.Endpoints[i], nil
		}
	}
	return nil, &TableNotFoundError{&Error{Message: "GridAPI: table not found: " + table}}
}

// GetMe returns the authenticated account and subscription summary.
func (c *Client) GetMe(ctx context.Context) (*Account, error) {
	raw, err := c.do(ctx, http.MethodGet, pathMe, nil)
	if err != nil {
		return nil, err
	}
	return decodeJSON[Account](raw)
}

// GetUsage returns the account's API call counters.
func (c *Client) GetUsage(ctx context.Context) (*Usage, error) {
	raw, err := c.do(ctx, http.MethodGet, pathMeUsage, nil)
	if err != nil {
		return nil, err
	}
	return decodeJSON[Usage](raw)
}

type projectsResponse struct {
	Projects []Project `json:"projects"`
}

type docsResponse struct {
	Endpoints []TableSchema `json:"endpoints"`
}

func dataPath(project, table string) string {
	p := strings.ReplaceAll(pathRecords, "{projectSlug}", url.PathEscape(project))
	return strings.ReplaceAll(p, "{endpoint}", url.PathEscape(table))
}

func recordPath(project, table, id string) string {
	p := strings.ReplaceAll(pathRecord, "{projectSlug}", url.PathEscape(project))
	p = strings.ReplaceAll(p, "{endpoint}", url.PathEscape(table))
	return strings.ReplaceAll(p, "{id}", url.PathEscape(id))
}

func docsPath(project string) string {
	return strings.ReplaceAll(pathProjectDocs, "{projectRef}", url.PathEscape(project))
}

func buildQuery(params *QueryOptions) string {
	if params == nil {
		return ""
	}
	q := url.Values{}
	switch f := params.Filter.(type) {
	case string:
		if f != "" {
			q.Set("filter", f)
		}
	case map[string]string:
		if len(f) > 0 {
			pairs := make([]string, 0, len(f))
			for k, v := range f {
				pairs = append(pairs, k+":"+v)
			}
			sort.Strings(pairs)
			q.Set("filter", strings.Join(pairs, ","))
		}
	}
	if params.Page > 0 {
		q.Set("page", strconv.Itoa(params.Page))
	}
	if params.Limit > 0 {
		q.Set("limit", strconv.Itoa(params.Limit))
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

func decodeJSON[T any](raw []byte) (*T, error) {
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, &Error{Message: "GridAPI: invalid response: " + err.Error()}
	}
	return &out, nil
}

// do performs a request with per-attempt timeouts and retries for transient
// failures on idempotent methods. POST is never retried.
func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, &ValidationError{&Error{Message: c.redact("GridAPI: invalid request body: " + err.Error())}}
		}
	}

	safe := method == http.MethodGet || method == http.MethodDelete || method == http.MethodPut ||
		(method == http.MethodPatch && c.retryWrites)
	fullURL := c.baseURL + path

	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, c.timeoutError(err)
		}
		reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
		req, err := http.NewRequestWithContext(reqCtx, method, fullURL, bytes.NewReader(payload))
		if err != nil {
			cancel()
			return nil, &ValidationError{&Error{Message: c.redact("GridAPI: invalid request: " + err.Error())}}
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			cancel()
			apiErr := c.transportError(err, reqCtx.Err())
			if safe && attempt < c.maxRetries && sleepCtx(ctx, retryDelay("", c.retryDelay, attempt)) {
				continue
			}
			return nil, apiErr
		}

		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		cancel()
		if readErr != nil {
			apiErr := &NetworkError{&Error{
				Message:   c.redact("GridAPI network error: " + readErr.Error()),
				Retryable: true,
			}}
			if safe && attempt < c.maxRetries && sleepCtx(ctx, retryDelay("", c.retryDelay, attempt)) {
				continue
			}
			return nil, apiErr
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return raw, nil
		}

		apiErr := c.mapHTTPError(resp.StatusCode, raw)
		if safe && attempt < c.maxRetries && retryable(apiErr) &&
			sleepCtx(ctx, retryDelay(resp.Header.Get("Retry-After"), c.retryDelay, attempt)) {
			continue
		}
		return nil, apiErr
	}
}

func (c *Client) timeoutError(err error) error {
	return &TimeoutError{&Error{
		Message:   c.redact("GridAPI request cancelled: " + err.Error()),
		Retryable: true,
	}}
}

func (c *Client) transportError(err, ctxErr error) error {
	if isTimeout(err) || errors.Is(ctxErr, context.DeadlineExceeded) {
		return &TimeoutError{&Error{
			Message:   c.redact("GridAPI request timed out: " + err.Error()),
			Retryable: true,
		}}
	}
	return &NetworkError{&Error{
		Message:   c.redact("GridAPI network error: " + err.Error()),
		Retryable: true,
	}}
}

func (c *Client) mapHTTPError(status int, raw []byte) error {
	message, code := parseErrorBody(raw, status)
	base := &Error{
		Status:    status,
		Code:      code,
		Message:   c.redact(message),
		Body:      c.redact(string(raw)),
		Retryable: status >= 500,
	}
	switch {
	case status == http.StatusUnauthorized:
		return &AuthenticationError{base}
	case status == http.StatusForbidden:
		return &AuthorizationError{base}
	case status == http.StatusNotFound:
		switch {
		case code == "SHEET_NOT_FOUND" || sheetWord.MatchString(message):
			return &TableNotFoundError{base}
		case code == "NOT_FOUND" || recordWord.MatchString(message):
			return &RecordNotFoundError{base}
		default:
			return &ProjectNotFoundError{base}
		}
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		return &ValidationError{base}
	case status == http.StatusTooManyRequests:
		if code == "SUBSCRIPTION_LIMIT_REACHED" {
			base.Retryable = false
			return &QuotaExceededError{base}
		}
		base.Retryable = true
		return &RateLimitError{base}
	case status >= 502:
		base.Retryable = true
		return &UpstreamError{base}
	default:
		return base
	}
}

func parseErrorBody(raw []byte, status int) (message, code string) {
	message = fmt.Sprintf("GridAPI request failed with status %d", status)
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		if len(raw) > 0 {
			message = string(raw)
		}
		return message, code
	}
	if s, ok := obj["error"].(string); ok {
		message = s
	} else if s, ok := obj["message"].(string); ok {
		message = s
	}
	code, _ = obj["code"].(string)
	return message, code
}

func retryable(err error) bool {
	var api APIError
	return errors.As(err, &api) && api.IsRetryable()
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout())
}

// retryDelay honours Retry-After in seconds or as an HTTP date, then falls
// back to exponential backoff with jitter capped at 30s, as in the JS SDK.
func retryDelay(retryAfter string, base time.Duration, attempt int) time.Duration {
	if retryAfter != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil {
			if secs < 0 {
				secs = 0
			}
			return time.Duration(secs) * time.Second
		}
		if at, err := http.ParseTime(retryAfter); err == nil {
			if d := time.Until(at); d > 0 {
				return d
			}
			return 0
		}
	}
	shift := attempt
	if shift > 20 {
		shift = 20
	}
	delay := base * time.Duration(1<<shift)
	if delay > 30*time.Second || delay < 0 {
		delay = 30 * time.Second
	}
	if jitterMax := int64(base / 2); jitterMax > 0 {
		delay += time.Duration(rand.Int63n(jitterMax))
	}
	return delay
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

var (
	bearerPattern = regexp.MustCompile(`(?i)Bearer\s+\S+`)
	keyPattern    = regexp.MustCompile(`(gp_live_|gapi_live_)[A-Za-z0-9_-]+`)
	sheetWord     = regexp.MustCompile(`(?i)sheet|table`)
	recordWord    = regexp.MustCompile(`(?i)record`)
)

func redact(s string) string {
	s = bearerPattern.ReplaceAllString(s, "Bearer [REDACTED]")
	return keyPattern.ReplaceAllString(s, "${1}[REDACTED]")
}

func (c *Client) redact(s string) string {
	if c != nil {
		if c.apiKey != "" {
			s = strings.ReplaceAll(s, c.apiKey, "[REDACTED]")
		}
		if c.projectKey != "" {
			s = strings.ReplaceAll(s, c.projectKey, "[REDACTED]")
		}
	}
	return redact(s)
}

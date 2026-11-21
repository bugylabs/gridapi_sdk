//! Official Rust SDK for the GridAPI public REST API.
//!
//! One account-level API key authenticates every authorized project. A project
//! slug is a routing/authorization target, never a credential.
//!
//! # Server-side only
//!
//! The API key is a secret. Never embed it in a browser bundle, mobile app, or
//! any other client that ships to end users. Keep it in an environment variable
//! on your server.
//!
//! # Quick start
//!
//! ```no_run
//! use gridapi_sdk::{Client, QueryOptions};
//!
//! # fn main() -> Result<(), gridapi_sdk::GridError> {
//! // Reads GRIDAPI_API_KEY, GRIDAPI_BASE_URL, GRIDAPI_TIMEOUT_MS from the env.
//! let client = Client::from_env()?;
//! let options = QueryOptions {
//!     filter: Some("status:active".into()),
//!     limit: Some(10),
//!     ..Default::default()
//! };
//! let page = client.get("acme", "users", &options)?;
//! println!("{} of {} rows", page.data.len(), page.meta.total);
//! # Ok(())
//! # }
//! ```

use std::collections::HashMap;
use std::env;
use std::fmt;
use std::thread;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};

/// A single row: column name to value (or `null` for an empty cell).
pub type GridRecord = HashMap<String, Option<String>>;

/// Exact-match filters encoded as `"col:value,col2:value2"`.
pub type Filter = String;

// ---------------------------------------------------------------------------
// Response shapes
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ListMeta {
    pub total: u64,
    pub limit: u64,
    pub offset: u64,
    #[serde(rename = "cacheHit", default, skip_serializing_if = "Option::is_none")]
    pub cache_hit: Option<bool>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ListResponse<T = GridRecord> {
    #[serde(default)]
    pub data: Vec<T>,
    #[serde(default)]
    pub meta: ListMeta,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct CreateResponse<T = GridRecord> {
    pub inserted: u64,
    pub records: Vec<T>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct UpdateResponse<T = GridRecord> {
    pub updated: u64,
    pub record: T,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct DeleteResponse {
    pub deleted: u64,
    pub id: String,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SheetInfo {
    pub endpoint: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    #[serde(rename = "sheetName", default, skip_serializing_if = "Option::is_none")]
    pub sheet_name: Option<String>,
    #[serde(rename = "primaryKey", default, skip_serializing_if = "Option::is_none")]
    pub primary_key: Option<String>,
    #[serde(rename = "allowedMethods", default, skip_serializing_if = "Option::is_none")]
    pub allowed_methods: Option<Vec<String>>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ProjectInfo {
    pub id: String,
    pub name: String,
    pub slug: String,
    pub role: String,
    #[serde(rename = "spreadsheetId", default, skip_serializing_if = "Option::is_none")]
    pub spreadsheet_id: Option<String>,
    #[serde(default)]
    pub sheets: Vec<SheetInfo>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SchemaField {
    pub name: String,
    #[serde(rename = "type", default, skip_serializing_if = "Option::is_none")]
    pub field_type: Option<String>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TableSchema {
    pub endpoint: String,
    #[serde(rename = "primaryKey", default, skip_serializing_if = "Option::is_none")]
    pub primary_key: Option<String>,
    #[serde(default)]
    pub fields: Vec<SchemaField>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct AccountDetail {
    #[serde(default)]
    pub id: Option<String>,
    #[serde(default)]
    pub email: Option<String>,
    #[serde(default)]
    pub scope: Option<String>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct AccountInfo {
    pub authenticated: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub account: Option<AccountDetail>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub subscription: Option<serde_json::Value>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct UsageInfo {
    #[serde(rename = "totalApiCallsUsed", default)]
    pub total_api_calls_used: Option<u64>,
    #[serde(rename = "totalApiCallsLeft", default)]
    pub total_api_calls_left: Option<u64>,
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

/// Shared error payload carried by every [`GridError`] variant.
#[derive(Debug, Clone)]
pub struct ErrorDetail {
    pub status: Option<u16>,
    pub code: Option<String>,
    /// Human-readable message. Always redacted before it reaches this struct.
    pub message: String,
}

/// Errors mirroring the TypeScript SDK's taxonomy.
#[derive(Debug, Clone)]
pub enum GridError {
    Authentication(ErrorDetail),
    Authorization(ErrorDetail),
    ProjectNotFound(ErrorDetail),
    TableNotFound(ErrorDetail),
    RecordNotFound(ErrorDetail),
    Validation(ErrorDetail),
    RateLimit(ErrorDetail),
    QuotaExceeded(ErrorDetail),
    Upstream(ErrorDetail),
    Network(ErrorDetail),
    Timeout(ErrorDetail),
}

impl GridError {
    /// HTTP status that produced this error, if any.
    pub fn status(&self) -> Option<u16> {
        self.detail().status
    }

    /// Backend error code, if any.
    pub fn code(&self) -> Option<&str> {
        self.detail().code.as_deref()
    }

    /// Redacted, human-readable message.
    pub fn message(&self) -> &str {
        &self.detail().message
    }

    /// Whether a retry may succeed. Callers still honour method idempotency.
    pub fn retryable(&self) -> bool {
        matches!(
            self,
            Self::RateLimit(_) | Self::Upstream(_) | Self::Network(_) | Self::Timeout(_)
        )
    }

    fn detail(&self) -> &ErrorDetail {
        match self {
            Self::Authentication(d)
            | Self::Authorization(d)
            | Self::ProjectNotFound(d)
            | Self::TableNotFound(d)
            | Self::RecordNotFound(d)
            | Self::Validation(d)
            | Self::RateLimit(d)
            | Self::QuotaExceeded(d)
            | Self::Upstream(d)
            | Self::Network(d)
            | Self::Timeout(d) => d,
        }
    }
}

impl fmt::Display for GridError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let d = self.detail();
        write!(f, "{}", d.message)?;
        if let Some(status) = d.status {
            write!(f, " (status {status})")?;
        }
        if let Some(code) = &d.code {
            write!(f, " [{code}]")?;
        }
        Ok(())
    }
}

impl std::error::Error for GridError {}

/// Remove credential-looking text from a string before it is logged or shown.
pub fn redact(input: &str) -> String {
    fn starts(haystack: &[u8], at: usize, needle: &[u8]) -> bool {
        haystack.get(at..at + needle.len()) == Some(needle)
    }
    fn token_byte(b: u8) -> bool {
        b.is_ascii_alphanumeric() || b == b'_' || b == b'-'
    }

    let bytes = input.as_bytes();
    let mut out = String::with_capacity(input.len());
    let mut i = 0;
    while i < bytes.len() {
        // Account key prefixes: gp_live_… / gapi_live_…
        let prefix = if starts(bytes, i, b"gapi_live_") {
            Some("gapi_live_".len())
        } else if starts(bytes, i, b"gp_live_") {
            Some("gp_live_".len())
        } else {
            None
        };
        if let Some(n) = prefix {
            out.push_str(&input[i..i + n]);
            out.push_str("[REDACTED]");
            i += n;
            while i < bytes.len() && token_byte(bytes[i]) {
                i += 1;
            }
            continue;
        }
        // Authorization header value: "Bearer <token>".
        if i + 6 <= bytes.len()
            && bytes[i..i + 6].eq_ignore_ascii_case(b"bearer")
            && i + 6 < bytes.len()
            && (bytes[i + 6] as char).is_ascii_whitespace()
        {
            let mut j = i + 6;
            while j < bytes.len() && (bytes[j] as char).is_ascii_whitespace() {
                j += 1;
            }
            let start = j;
            while j < bytes.len() && !(bytes[j] as char).is_ascii_whitespace() {
                j += 1;
            }
            if j > start {
                out.push_str("Bearer [REDACTED]");
                i = j;
                continue;
            }
        }
        let ch = input[i..].chars().next().expect("non-empty UTF-8 slice");
        out.push(ch);
        i += ch.len_utf8();
    }
    out
}

fn detail(status: u16, code: Option<&str>, message: &str) -> ErrorDetail {
    ErrorDetail {
        status: Some(status),
        code: code.map(str::to_string),
        message: redact(message),
    }
}

fn with_no_status(message: &str) -> ErrorDetail {
    ErrorDetail {
        status: None,
        code: None,
        message: redact(message),
    }
}

fn decode_error(err: &serde_json::Error) -> GridError {
    GridError::Upstream(with_no_status(&format!(
        "GridAPI: failed to decode response: {err}"
    )))
}

/// Map an HTTP status plus backend body to the SDK's error taxonomy.
fn map_status(status: u16, body: &str) -> GridError {
    let parsed: Option<serde_json::Value> = serde_json::from_str(body).ok();
    let code = parsed
        .as_ref()
        .and_then(|v| v.get("code"))
        .and_then(|v| v.as_str())
        .map(str::to_string);
    let message = parsed
        .as_ref()
        .and_then(|v| {
            v.get("error")
                .or_else(|| v.get("message"))
                .and_then(|v| v.as_str())
        })
        .filter(|s| !s.is_empty())
        .map(str::to_string)
        .unwrap_or_else(|| format!("GridAPI request failed with status {status}"));
    let code = code.as_deref();
    let lower = message.to_ascii_lowercase();

    if status == 401 {
        return GridError::Authentication(detail(status, code, &message));
    }
    if status == 403 {
        return GridError::Authorization(detail(status, code, &message));
    }
    if status == 404 {
        if code == Some("SHEET_NOT_FOUND") || lower.contains("sheet") || lower.contains("table") {
            return GridError::TableNotFound(detail(status, code, &message));
        }
        if code == Some("NOT_FOUND") || lower.contains("record") {
            return GridError::RecordNotFound(detail(status, code, &message));
        }
        return GridError::ProjectNotFound(detail(status, code, &message));
    }
    if (400..=422).contains(&status) {
        return GridError::Validation(detail(status, code, &message));
    }
    if status == 429 {
        return if code == Some("SUBSCRIPTION_LIMIT_REACHED") {
            GridError::QuotaExceeded(detail(status, code, &message))
        } else {
            GridError::RateLimit(detail(status, code, &message))
        };
    }
    if status >= 500 {
        return GridError::Upstream(detail(status, code, &message));
    }
    GridError::Validation(detail(status, code, &message))
}

// ---------------------------------------------------------------------------
// Timing helpers
// ---------------------------------------------------------------------------

/// Parse an IMF-fixdate such as `Sun, 06 Nov 1994 08:49:37 GMT` into epoch ms.
fn parse_http_date(value: &str) -> Option<u64> {
    let rest = value.trim().split_once(',')?.1.trim_start();
    let parts: Vec<&str> = rest.split_whitespace().collect();
    if parts.len() != 5 || parts[4] != "GMT" {
        return None;
    }
    let day: i64 = parts[0].parse().ok()?;
    let month: i64 = match parts[1] {
        "Jan" => 1,
        "Feb" => 2,
        "Mar" => 3,
        "Apr" => 4,
        "May" => 5,
        "Jun" => 6,
        "Jul" => 7,
        "Aug" => 8,
        "Sep" => 9,
        "Oct" => 10,
        "Nov" => 11,
        "Dec" => 12,
        _ => return None,
    };
    let year: i64 = parts[2].parse().ok()?;
    let clock: Vec<i64> = parts[3]
        .split(':')
        .map(str::parse::<i64>)
        .collect::<Result<_, _>>()
        .ok()?;
    if clock.len() != 3 {
        return None;
    }
    // Days from the Unix epoch (Howard Hinnant's civil-date algorithm).
    let y = year - if month <= 2 { 1 } else { 0 };
    let era = if y >= 0 { y } else { y - 399 } / 400;
    let yoe = y - era * 400;
    let mp = month + if month > 2 { -3 } else { 9 };
    let doy = (153 * mp + 2) / 5 + day - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    let days = era * 146097 + doe - 719468;
    let secs = days * 86_400 + clock[0] * 3600 + clock[1] * 60 + clock[2];
    u64::try_from(secs).ok().map(|s| s.saturating_mul(1000))
}

fn retry_delay(retry_after: Option<&str>, base_ms: u64, attempt: u32) -> Duration {
    if let Some(value) = retry_after {
        let value = value.trim();
        if let Ok(seconds) = value.parse::<u64>() {
            return Duration::from_millis(seconds.saturating_mul(1000));
        }
        if let Some(target_ms) = parse_http_date(value) {
            let now = SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap_or_default()
                .as_millis() as u64;
            return Duration::from_millis(target_ms.saturating_sub(now));
        }
    }
    // Exponential backoff with a small time-seeded jitter, capped at 30s.
    let exponential = base_ms.saturating_mul(1u64 << attempt.min(12)).min(30_000);
    let seed = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .subsec_nanos() as u64;
    let jitter = seed % (base_ms / 2 + 1);
    Duration::from_millis(exponential.saturating_add(jitter))
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

#[derive(Clone, Copy)]
enum Method {
    Get,
    Post,
    Patch,
    Put,
    Delete,
}

impl Method {
    /// POST is never retried; PATCH only when explicitly opted in.
    fn retryable(self, retry_writes: bool) -> bool {
        match self {
            Self::Get | Self::Delete | Self::Put => true,
            Self::Patch => retry_writes,
            Self::Post => false,
        }
    }
}

struct RawResponse {
    status: u16,
    body: String,
    retry_after: Option<String>,
}

/// Options applied to `get` / `query` / `count`.
///
/// Sorting and search are intentionally absent: the live data source ignores
/// them, so the SDK does not pretend to support them.
#[derive(Debug, Clone, Default)]
pub struct QueryOptions {
    pub filter: Option<Filter>,
    /// 1-based page number; applied by the server only when `limit` is set.
    pub page: Option<u32>,
    /// Page size, clamped to 1–100 by the server.
    pub limit: Option<u32>,
}

fn query_pairs(options: &QueryOptions) -> Vec<(&'static str, String)> {
    let mut pairs = Vec::new();
    if let Some(filter) = &options.filter {
        pairs.push(("filter", filter.clone()));
    }
    if let Some(page) = options.page {
        pairs.push(("page", page.to_string()));
    }
    if let Some(limit) = options.limit {
        pairs.push(("limit", limit.to_string()));
    }
    pairs
}

fn encode_segment(value: &str) -> String {
    let mut out = String::with_capacity(value.len());
    for &b in value.as_bytes() {
        match b {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'_' | b'.' | b'~' => {
                out.push(b as char)
            }
            _ => out.push_str(&format!("%{b:02X}")),
        }
    }
    out
}

fn data_path(project: &str, table: &str) -> String {
    format!("/api/v1/{}/{}", encode_segment(project), encode_segment(table))
}

fn data_path_with_id(project: &str, table: &str, id: &str) -> String {
    format!(
        "/api/v1/{}/{}/{}",
        encode_segment(project),
        encode_segment(table),
        encode_segment(id)
    )
}

/// Builder for [`Client`]. Any value not set explicitly is read from the
/// environment when [`build`](ClientBuilder::build) is called.
#[derive(Debug, Clone)]
pub struct ClientBuilder {
    api_key: Option<String>,
    base_url: Option<String>,
    timeout_ms: Option<u64>,
    max_retries: usize,
    retry_delay_ms: u64,
    retry_writes: bool,
}

impl Default for ClientBuilder {
    fn default() -> Self {
        Self::new()
    }
}

impl ClientBuilder {
    pub fn new() -> Self {
        Self {
            api_key: None,
            base_url: None,
            timeout_ms: None,
            max_retries: 2,
            retry_delay_ms: 500,
            retry_writes: false,
        }
    }

    /// Account-level API key. Defaults to `GRIDAPI_API_KEY` (required).
    pub fn api_key(mut self, key: impl Into<String>) -> Self {
        self.api_key = Some(key.into());
        self
    }

    /// API root. Defaults to `GRIDAPI_BASE_URL` or `https://api.gridapi.in`.
    pub fn base_url(mut self, url: impl Into<String>) -> Self {
        self.base_url = Some(url.into());
        self
    }

    /// Per-request timeout. Defaults to `GRIDAPI_TIMEOUT_MS` or 30000.
    pub fn timeout_ms(mut self, ms: u64) -> Self {
        self.timeout_ms = Some(ms);
        self
    }

    /// Retry attempts after the first request. Default 2.
    pub fn max_retries(mut self, retries: usize) -> Self {
        self.max_retries = retries;
        self
    }

    /// Base backoff in milliseconds. Default 500.
    pub fn retry_delay_ms(mut self, ms: u64) -> Self {
        self.retry_delay_ms = ms;
        self
    }

    /// Retry non-idempotent PATCH writes. Default false. POST is never retried.
    pub fn retry_writes(mut self, enabled: bool) -> Self {
        self.retry_writes = enabled;
        self
    }

    pub fn build(self) -> Result<Client, GridError> {
        let api_key = self
            .api_key
            .or_else(|| env::var("GRIDAPI_API_KEY").ok())
            .filter(|key| !key.is_empty())
            .ok_or_else(|| {
                GridError::Authentication(with_no_status(
                    "GridAPI: missing API key. Set GRIDAPI_API_KEY or use ClientBuilder::api_key().",
                ))
            })?;

        let base_url = self
            .base_url
            .or_else(|| env::var("GRIDAPI_BASE_URL").ok())
            .unwrap_or_else(|| "https://api.gridapi.in".to_string())
            .trim_end_matches('/')
            .to_string();

        let timeout_ms = self
            .timeout_ms
            .or_else(|| {
                env::var("GRIDAPI_TIMEOUT_MS")
                    .ok()
                    .and_then(|value| value.parse::<u64>().ok())
            })
            .unwrap_or(30_000);

        let agent = ureq::AgentBuilder::new()
            .timeout(Duration::from_millis(timeout_ms))
            .build();

        Ok(Client {
            agent,
            bearer: format!("Bearer {api_key}"),
            base_url,
            max_retries: self.max_retries,
            retry_delay_ms: self.retry_delay_ms,
            retry_writes: self.retry_writes,
        })
    }
}

/// Synchronous GridAPI client. Build one with [`Client::from_env`] or
/// [`ClientBuilder`], then reuse it across requests (it holds a pooled agent).
pub struct Client {
    agent: ureq::Agent,
    bearer: String,
    base_url: String,
    max_retries: usize,
    retry_delay_ms: u64,
    retry_writes: bool,
}

impl Client {
    /// Build a client from `GRIDAPI_API_KEY`, `GRIDAPI_BASE_URL` and
    /// `GRIDAPI_TIMEOUT_MS`.
    pub fn from_env() -> Result<Self, GridError> {
        ClientBuilder::new().build()
    }

    fn send_once(
        &self,
        method: Method,
        path: &str,
        query: &[(&str, String)],
        body: Option<&str>,
    ) -> Result<RawResponse, GridError> {
        let url = format!("{}{}", self.base_url, path);
        let mut request = match method {
            Method::Get => self.agent.get(&url),
            Method::Post => self.agent.post(&url),
            Method::Patch => self.agent.patch(&url),
            Method::Put => self.agent.put(&url),
            Method::Delete => self.agent.delete(&url),
        };
        request = request
            .set("Authorization", &self.bearer)
            .set("Accept", "application/json")
            .set("Content-Type", "application/json");
        for (name, value) in query {
            request = request.query(name, value);
        }

        let result = match body {
            Some(payload) => request.send_string(payload),
            None => request.call(),
        };

        match result {
            Ok(response) => Ok(RawResponse {
                status: response.status(),
                retry_after: response.header("retry-after").map(str::to_string),
                body: response.into_string().unwrap_or_default(),
            }),
            Err(ureq::Error::Status(status, response)) => Ok(RawResponse {
                status,
                retry_after: response.header("retry-after").map(str::to_string),
                body: response.into_string().unwrap_or_default(),
            }),
            Err(ureq::Error::Transport(transport)) => {
                let text = transport.to_string().to_lowercase();
                let message = format!("GridAPI network error: {transport}");
                if text.contains("timed out") || text.contains("timeout") || text.contains("too slow") {
                    Err(GridError::Timeout(with_no_status(&message)))
                } else {
                    Err(GridError::Network(with_no_status(&message)))
                }
            }
        }
    }

    fn request(
        &self,
        method: Method,
        path: String,
        query: Vec<(&'static str, String)>,
        body: Option<String>,
    ) -> Result<String, GridError> {
        let attempts = self.max_retries + 1;
        let mut last_error: Option<GridError> = None;
        for attempt in 0..attempts {
            match self.send_once(method, &path, &query, body.as_deref()) {
                Ok(response) if (200..300).contains(&response.status) => return Ok(response.body),
                Ok(response) => {
                    let error = map_status(response.status, &response.body);
                    let can_retry = method.retryable(self.retry_writes)
                        && error.retryable()
                        && attempt + 1 < attempts;
                    if !can_retry {
                        return Err(error);
                    }
                    last_error = Some(error);
                    thread::sleep(retry_delay(
                        response.retry_after.as_deref(),
                        self.retry_delay_ms,
                        attempt as u32,
                    ));
                }
                Err(error) => {
                    let can_retry = method.retryable(self.retry_writes)
                        && error.retryable()
                        && attempt + 1 < attempts;
                    if !can_retry {
                        return Err(error);
                    }
                    last_error = Some(error);
                    thread::sleep(retry_delay(None, self.retry_delay_ms, attempt as u32));
                }
            }
        }
        Err(last_error.expect("at least one attempt has produced an error"))
    }

    /// List records. Sorting and search are deliberately unsupported.
    pub fn get(
        &self,
        project: &str,
        table: &str,
        options: &QueryOptions,
    ) -> Result<ListResponse, GridError> {
        let body = self.request(
            Method::Get,
            data_path(project, table),
            query_pairs(options),
            None,
        )?;
        serde_json::from_str(&body).map_err(|err| decode_error(&err))
    }

    /// Alias of [`get`](Client::get), for callers that prefer `query`.
    pub fn query(
        &self,
        project: &str,
        table: &str,
        options: &QueryOptions,
    ) -> Result<ListResponse, GridError> {
        self.get(project, table, options)
    }

    /// Count matching records using `limit=1` and the list response's `meta.total`.
    pub fn count(&self, project: &str, table: &str, filter: Option<&str>) -> Result<u64, GridError> {
        let options = QueryOptions {
            filter: filter.map(str::to_string),
            limit: Some(1),
            ..Default::default()
        };
        Ok(self.get(project, table, &options)?.meta.total)
    }

    /// Retrieve one record by its primary key.
    pub fn get_by_id(&self, project: &str, table: &str, id: &str) -> Result<GridRecord, GridError> {
        let body = self.request(Method::Get, data_path_with_id(project, table, id), vec![], None)?;
        serde_json::from_str(&body).map_err(|err| decode_error(&err))
    }

    /// Insert one or more records. This is a write and is never retried.
    pub fn post(&self, project: &str, table: &str, records: &[GridRecord]) -> Result<CreateResponse, GridError> {
        let payload = serde_json::to_string(records).map_err(|err| decode_error(&err))?;
        let body = self.request(Method::Post, data_path(project, table), vec![], Some(payload))?;
        serde_json::from_str(&body).map_err(|err| decode_error(&err))
    }

    /// Partially update a record. Retried only when `retry_writes` is enabled.
    pub fn patch(
        &self,
        project: &str,
        table: &str,
        id: &str,
        data: &GridRecord,
    ) -> Result<UpdateResponse, GridError> {
        let payload = serde_json::to_string(data).map_err(|err| decode_error(&err))?;
        let body = self.request(
            Method::Patch,
            data_path_with_id(project, table, id),
            vec![],
            Some(payload),
        )?;
        serde_json::from_str(&body).map_err(|err| decode_error(&err))
    }

    /// Replace a record.
    pub fn put(
        &self,
        project: &str,
        table: &str,
        id: &str,
        data: &GridRecord,
    ) -> Result<UpdateResponse, GridError> {
        let payload = serde_json::to_string(data).map_err(|err| decode_error(&err))?;
        let body = self.request(
            Method::Put,
            data_path_with_id(project, table, id),
            vec![],
            Some(payload),
        )?;
        serde_json::from_str(&body).map_err(|err| decode_error(&err))
    }

    /// Delete a record by its primary key.
    pub fn delete(&self, project: &str, table: &str, id: &str) -> Result<DeleteResponse, GridError> {
        let body = self.request(Method::Delete, data_path_with_id(project, table, id), vec![], None)?;
        serde_json::from_str(&body).map_err(|err| decode_error(&err))
    }

    /// Projects visible to this account key (discovery read, no quota cost).
    pub fn list_projects(&self) -> Result<Vec<ProjectInfo>, GridError> {
        let body = self.request(Method::Get, "/api/v1/me/projects".to_string(), vec![], None)?;
        #[derive(Deserialize)]
        struct Wrapper {
            #[serde(default)]
            projects: Vec<ProjectInfo>,
        }
        let wrapper: Wrapper = serde_json::from_str(&body).map_err(|err| decode_error(&err))?;
        Ok(wrapper.projects)
    }

    /// Sheets configured for a project, derived from the projects discovery read.
    pub fn list_tables(&self, project: &str) -> Result<Vec<SheetInfo>, GridError> {
        let projects = self.list_projects()?;
        let found = projects
            .iter()
            .find(|candidate| {
                candidate.slug == project || candidate.id == project || candidate.name == project
            })
            .ok_or_else(|| {
                GridError::ProjectNotFound(detail(
                    404,
                    None,
                    &format!("Project not found: {project}"),
                ))
            })?;
        Ok(found.sheets.clone())
    }

    /// Field metadata for one table, derived from the project's docs endpoint.
    pub fn get_schema(&self, project: &str, table: &str) -> Result<TableSchema, GridError> {
        let path = format!("/api/v1/me/projects/{}/docs", encode_segment(project));
        let body = self.request(Method::Get, path, vec![], None)?;
        #[derive(Deserialize)]
        struct Wrapper {
            #[serde(default)]
            endpoints: Vec<TableSchema>,
        }
        let wrapper: Wrapper = serde_json::from_str(&body).map_err(|err| decode_error(&err))?;
        wrapper
            .endpoints
            .into_iter()
            .find(|schema| schema.endpoint.eq_ignore_ascii_case(table))
            .ok_or_else(|| {
                GridError::TableNotFound(detail(404, None, &format!("Table not found: {table}")))
            })
    }

    /// Authenticated account and subscription summary.
    pub fn get_me(&self) -> Result<AccountInfo, GridError> {
        let body = self.request(Method::Get, "/api/v1/me".to_string(), vec![], None)?;
        serde_json::from_str(&body).map_err(|err| decode_error(&err))
    }

    /// Account quota usage.
    pub fn get_usage(&self) -> Result<UsageInfo, GridError> {
        let body = self.request(Method::Get, "/api/v1/me/usage".to_string(), vec![], None)?;
        serde_json::from_str(&body).map_err(|err| decode_error(&err))
    }
}

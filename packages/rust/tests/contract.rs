//! Contract tests for the Rust SDK. Each test boots the shared, in-memory mock
//! server (`tests/contract/mock-server.mjs`) on its own port and kills it on drop.

use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::thread;
use std::time::Duration;

use gridapi_sdk::{redact, Client, ClientBuilder, GridError, QueryOptions};

const KEY: &str = "gp_live_testkey";
const OTHER_KEY: &str = "gp_live_otheraccount";

fn repo_root() -> PathBuf {
    let manifest = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    manifest
        .parent()
        .and_then(|p| p.parent())
        .expect("packages/rust has a parent")
        .to_path_buf()
}

struct MockServer {
    child: Child,
    base: String,
}

impl MockServer {
    // Drop kills and waits for the child on every path.
    #[allow(clippy::zombie_processes)]
    fn start(port: u16) -> Self {
        let script = repo_root().join("tests/contract/mock-server.mjs");
        let child = Command::new("node")
            .arg(&script)
            .arg(port.to_string())
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .expect("start mock server with node");
        let base = format!("http://127.0.0.1:{port}");
        for _ in 0..100 {
            if TcpStream::connect(("127.0.0.1", port)).is_ok() {
                return Self { child, base };
            }
            thread::sleep(Duration::from_millis(50));
        }
        panic!("mock server on port {port} never became ready");
    }

    fn client(&self) -> Client {
        ClientBuilder::new()
            .api_key(KEY)
            .base_url(&self.base)
            .build()
            .unwrap()
    }

    fn at(&self, suffix: &str) -> Client {
        ClientBuilder::new()
            .api_key(KEY)
            .base_url(format!("{}{}", self.base, suffix))
            .retry_delay_ms(1)
            .build()
            .unwrap()
    }
}

impl Drop for MockServer {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

#[test]
fn crud_filter_pagination_and_count() {
    let server = MockServer::start(18795);
    let client = server.client();

    let all = client
        .get("acme", "users", &QueryOptions::default())
        .unwrap();
    assert_eq!(all.data.len(), 3);
    assert_eq!(all.meta.total, 3);
    assert_eq!(all.meta.limit, 100);

    let filtered = client
        .query(
            "acme",
            "users",
            &QueryOptions {
                filter: Some("team:growth".into()),
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(filtered.meta.total, 2);
    assert_eq!(filtered.data.len(), 2);

    let page = client
        .get(
            "acme",
            "users",
            &QueryOptions {
                page: Some(2),
                limit: Some(2),
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(page.meta.offset, 2);
    assert_eq!(page.data.len(), 1);
    assert_eq!(page.data[0].get("id").and_then(|v| v.as_deref()), Some("3"));

    assert_eq!(client.count("acme", "users", None).unwrap(), 3);
    assert_eq!(
        client
            .count("acme", "users", Some("status:active"))
            .unwrap(),
        2
    );

    let record = client.get_by_id("acme", "users", "1").unwrap();
    assert_eq!(record.get("name").and_then(|v| v.as_deref()), Some("Ann"));

    let mut new = gridapi_sdk::GridRecord::new();
    new.insert("name".into(), Some("Dee".into()));
    let created = client.post("acme", "users", &[new]).unwrap();
    assert_eq!(created.inserted, 1);
    let id = created.records[0]
        .get("id")
        .and_then(|v| v.as_deref())
        .unwrap()
        .to_string();

    let mut patch = gridapi_sdk::GridRecord::new();
    patch.insert("name".into(), Some("Dee2".into()));
    let patched = client.patch("acme", "users", &id, &patch).unwrap();
    assert_eq!(patched.updated, 1);
    assert_eq!(
        patched.record.get("name").and_then(|v| v.as_deref()),
        Some("Dee2")
    );

    let updated = client.put("acme", "users", &id, &patch).unwrap();
    assert_eq!(updated.updated, 1);

    let deleted = client.delete("acme", "users", &id).unwrap();
    assert_eq!(deleted.deleted, 1);
    assert_eq!(deleted.id, id);
    assert!(matches!(
        client.get_by_id("acme", "users", &id),
        Err(GridError::RecordNotFound(_))
    ));
}

#[test]
fn error_mapping() {
    let server = MockServer::start(18796);

    let bad_key = ClientBuilder::new()
        .api_key("not-a-real-key")
        .base_url(&server.base)
        .build()
        .unwrap();
    assert!(matches!(
        bad_key.get("acme", "users", &QueryOptions::default()),
        Err(GridError::Authentication(e)) if e.status == Some(401)
    ));

    let other = ClientBuilder::new()
        .api_key(OTHER_KEY)
        .base_url(&server.base)
        .build()
        .unwrap();
    assert!(matches!(
        other.get("acme", "users", &QueryOptions::default()),
        Err(GridError::Authorization(e)) if e.status == Some(403)
    ));

    let client = server.client();
    assert!(matches!(
        client.get("missing", "users", &QueryOptions::default()),
        Err(GridError::ProjectNotFound(_))
    ));
    assert!(matches!(
        client.get("acme", "nope", &QueryOptions::default()),
        Err(GridError::TableNotFound(_))
    ));
    assert!(matches!(
        client.get_by_id("acme", "users", "nope"),
        Err(GridError::RecordNotFound(_))
    ));

    let mut bad = gridapi_sdk::GridRecord::new();
    bad.insert("__bad".into(), Some("x".into()));
    assert!(matches!(
        client.post("acme", "users", &[bad]),
        Err(GridError::Validation(e)) if e.code.as_deref() == Some("UNKNOWN_COLUMN")
    ));

    let quota = server.at("/fault/quota");
    let err = quota
        .get("acme", "users", &QueryOptions::default())
        .unwrap_err();
    assert!(matches!(err, GridError::QuotaExceeded(_)));
    assert!(!err.retryable());

    let rate = ClientBuilder::new()
        .api_key(KEY)
        .base_url(format!("{}/fault/rate", server.base))
        .max_retries(0)
        .build()
        .unwrap();
    let err = rate
        .get("acme", "users", &QueryOptions::default())
        .unwrap_err();
    assert!(matches!(err, GridError::RateLimit(_)));
    assert!(err.retryable());
}

#[test]
fn redaction() {
    assert_eq!(
        redact("Authorization: Bearer supersecret gp_live_abcdefghijkl"),
        "Authorization: Bearer [REDACTED] gp_live_[REDACTED]"
    );

    let server = MockServer::start(18797);
    let echo = server.at("/fault/echo");
    let err = echo
        .get("acme", "users", &QueryOptions::default())
        .unwrap_err();
    let rendered = format!("{err}");
    assert!(rendered.contains("[REDACTED]"));
    assert!(!rendered.contains(KEY));
    assert!(!err.message().contains(KEY));
}

#[test]
fn retries_honour_method_and_retry_after() {
    let server = MockServer::start(18798);
    let flaky = server.at("/fault/rate");
    let page = flaky
        .get("acme", "users", &QueryOptions::default())
        .unwrap();
    assert_eq!(page.data.len(), 3);

    let always = ClientBuilder::new()
        .api_key(KEY)
        .base_url(format!("{}/fault/always-rate", server.base))
        .max_retries(1)
        .retry_delay_ms(1)
        .build()
        .unwrap();
    assert!(matches!(
        always.get("acme", "users", &QueryOptions::default()),
        Err(GridError::RateLimit(_))
    ));

    // POST is never retried: the first 429 is returned immediately.
    let post_server = MockServer::start(18799);
    let flaky_post = post_server.at("/fault/rate");
    let mut row = gridapi_sdk::GridRecord::new();
    row.insert("name".into(), Some("X".into()));
    assert!(matches!(
        flaky_post.post("acme", "users", &[row]),
        Err(GridError::RateLimit(_))
    ));
}

#[test]
fn discovery() {
    let server = MockServer::start(18800);
    let client = server.client();

    assert!(client.get_me().unwrap().authenticated);
    assert_eq!(client.get_usage().unwrap().total_api_calls_left, Some(900));

    let projects = client.list_projects().unwrap();
    assert_eq!(projects.len(), 2);
    assert_eq!(projects[0].slug, "acme");

    let sheets = client.list_tables("acme").unwrap();
    assert_eq!(sheets.len(), 2);
    assert_eq!(sheets[0].endpoint, "users");

    let schema = client.get_schema("acme", "users").unwrap();
    assert_eq!(schema.endpoint, "users");
    assert_eq!(schema.fields.len(), 2);
    assert_eq!(schema.primary_key.as_deref(), Some("id"));

    assert!(matches!(
        client.list_tables("missing"),
        Err(GridError::ProjectNotFound(_))
    ));
    assert!(matches!(
        client.get_schema("acme", "nope"),
        Err(GridError::TableNotFound(_))
    ));
}

#[test]
fn env_configuration() {
    let server = MockServer::start(18801);
    std::env::set_var("GRIDAPI_API_KEY", KEY);
    std::env::set_var("GRIDAPI_BASE_URL", &server.base);
    std::env::set_var("GRIDAPI_TIMEOUT_MS", "1500");

    let client = Client::from_env().unwrap();
    assert_eq!(
        client
            .get("acme", "users", &QueryOptions::default())
            .unwrap()
            .data
            .len(),
        3
    );

    // Explicit builder overrides win over the environment.
    let overridden = ClientBuilder::new().api_key(OTHER_KEY).build().unwrap();
    assert!(matches!(
        overridden.get("acme", "users", &QueryOptions::default()),
        Err(GridError::Authorization(_))
    ));
}

#[test]
fn upstream_maps_from_http_5xx() {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let port = listener.local_addr().unwrap().port();
    std::thread::spawn(move || {
        for _ in 0..5 {
            if let Ok((mut stream, _)) = listener.accept() {
                let mut buf = [0u8; 1024];
                let _ = stream.read(&mut buf);
                let body = r#"{"error":"provider down"}"#;
                let response = format!(
                    "HTTP/1.1 503 Service Unavailable\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                    body.len(),
                    body
                );
                let _ = stream.write_all(response.as_bytes());
            }
        }
    });

    let client = ClientBuilder::new()
        .api_key(KEY)
        .base_url(format!("http://127.0.0.1:{port}"))
        .max_retries(1)
        .retry_delay_ms(1)
        .build()
        .unwrap();
    let err = client
        .get("acme", "users", &QueryOptions::default())
        .unwrap_err();
    assert!(matches!(err, GridError::Upstream(_)));
    assert!(err.retryable());
    assert_eq!(err.status(), Some(503));
}

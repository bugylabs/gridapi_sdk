//! Runnable GridAPI example. Configuration comes only from the environment:
//!   GRIDAPI_API_KEY   (required)
//!   GRIDAPI_BASE_URL  (optional, default https://api.gridapi.in)
//!   GRIDAPI_TIMEOUT_MS (optional, default 30000)
//!
//! Run from this directory with: cargo run

use gridapi_sdk::{Client, GridRecord, QueryOptions, GridError};

fn main() -> Result<(), GridError> {
    let client = Client::from_env()?;

    let me = client.get_me()?;
    println!("authenticated: {}", me.authenticated);

    let options = QueryOptions {
        filter: Some("status:active".into()),
        limit: Some(10),
        ..Default::default()
    };
    let page = client.query("acme", "users", &options)?;
    println!("got {} of {} rows", page.data.len(), page.meta.total);

    let mut row = GridRecord::new();
    row.insert("name".into(), Some("Ann".into()));
    row.insert("email".into(), Some("ann@example.com".into()));
    let created = client.post("acme", "users", &[row])?;
    println!("inserted {} record(s)", created.inserted);

    Ok(())
}

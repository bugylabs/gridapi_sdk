import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { after, before, test } from "node:test";
import {
  GridApiClient,
  GridAPI,
  ConfigurationError,
  AuthenticationError,
  AuthorizationError,
  RecordNotFoundError,
  ProjectNotFoundError,
  ValidationError,
  QuotaExceededError,
  RateLimitError,
  TimeoutError,
  redact,
} from "../dist/esm/index.js";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");
const port = 8790;
const baseUrl = `http://127.0.0.1:${port}`;
const ACCOUNT_KEY = "gp_live_testkey";
const PROJECT_KEY = "gapi_live_testproject";
let server;

async function waitReady() {
  const deadline = Date.now() + 5000;
  while (Date.now() < deadline) {
    try {
      const r = await fetch(`${baseUrl}/api/v1/capabilities`);
      if (r.ok) return;
    } catch {
      // not up yet
    }
    await new Promise((r) => setTimeout(r, 50));
  }
  throw new Error("mock server did not start");
}

before(async () => {
  server = spawn(process.execPath, ["tests/contract/mock-server.mjs", String(port)], {
    cwd: root,
    stdio: "ignore",
  });
  await waitReady();
});

after(() => server?.kill());

function client(extra = {}) {
  return new GridApiClient({ apiKey: ACCOUNT_KEY, projectKey: PROJECT_KEY, baseUrl, ...extra });
}

test("exports alias and default-compatible class", () => {
  assert.equal(GridAPI, GridApiClient);
});

test("redact strips keys and bearer tokens", () => {
  const text = "failed with gp_live_supersecretkey123 and Bearer abc.def";
  const clean = redact(text);
  assert.ok(!clean.includes("supersecretkey123"));
  assert.ok(!clean.includes("abc.def"));
});

test("both keys are required, well-formed and never echoed", async () => {
  assert.throws(
    () => new GridApiClient({ apiKey: ACCOUNT_KEY, baseUrl }),
    ConfigurationError,
  );
  assert.throws(
    () => new GridApiClient({ apiKey: ACCOUNT_KEY, projectKey: "gp_live_wrongprefix", baseUrl }),
    ConfigurationError,
  );
  assert.throws(
    () => new GridApiClient({ apiKey: "gapi_live_wrongprefix", projectKey: PROJECT_KEY, baseUrl }),
    ConfigurationError,
  );
  const api = client();
  await api.verify("acme");
  await assert.rejects(() => api.verify("ghost"), ProjectNotFoundError);
  try {
    new GridApiClient({ apiKey: ACCOUNT_KEY, projectKey: "", baseUrl });
  } catch (error) {
    assert.ok(!String(error.message).includes("gapi_live_"));
    assert.ok(!String(error.message).includes("testproject"));
  }
});

test("list, exact filter and page+limit pagination", async () => {
  const api = client();
  const all = await api.get("acme", "users");
  assert.equal(all.data.length, 3);
  assert.equal(all.meta.total, 3);

  const filtered = await api.query("acme", "users", { filter: "status:active" });
  assert.deepEqual(filtered.data.map((r) => r.id), ["1", "2"]);
  assert.equal(filtered.meta.total, 2);

  const objectFilter = await api.query("acme", "users", { filter: { team: "growth" } });
  assert.equal(objectFilter.meta.total, 2);

  const page = await api.query("acme", "users", { page: 2, limit: 2 });
  assert.deepEqual(page.data.map((r) => r.id), ["3"]);
  assert.equal(page.meta.offset, 2);
  assert.equal(page.meta.limit, 2);
});

test("getById, count and discovery", async () => {
  const api = client();
  assert.equal((await api.getById("acme", "users", "2")).name, "Ben");
  assert.equal(await api.count("acme", "users", "status:invited"), 1);

  const me = await api.getMe();
  assert.equal(me.authenticated, true);
  assert.equal(me.account.scope, "ACCOUNT");
  assert.ok(me.subscription);

  const projects = await api.listProjects();
  assert.deepEqual(projects.map((p) => p.slug).sort(), ["acme", "beta"]);
  assert.equal(projects.find((p) => p.slug === "acme").role, "OWNER");

  const tables = await api.listTables("acme");
  assert.deepEqual(tables.map((t) => t.endpoint).sort(), ["empty", "users"]);

  const schema = await api.getSchema("acme", "users");
  assert.equal(schema.endpoint, "users");
  assert.ok(Array.isArray(schema.fields));
  assert.ok((await api.getUsage()) && typeof api.getUsage === "function");
});

test("CRUD writes return the documented shapes", async () => {
  const api = client();
  const created = await api.post("acme", "users", { name: "Dee", status: "active" });
  assert.equal(created.inserted, 1);
  assert.equal(created.records.length, 1);
  const id = created.records[0].id;

  const patched = await api.patch("acme", "users", id, { status: "invited" });
  assert.equal(patched.updated, 1);
  assert.equal(patched.record.status, "invited");

  const replaced = await api.put("acme", "users", id, { name: "Dee2" });
  assert.equal(replaced.record.name, "Dee2");

  const removed = await api.delete("acme", "users", id);
  assert.equal(removed.deleted, 1);
  assert.equal(removed.id, id);
});

test("error mapping: auth, project, record, validation, quota", async () => {
  assert.equal((await client().get("acme", "users")).data.length, 3);

  assert.throws(
    () => new GridApiClient({ apiKey: "", projectKey: PROJECT_KEY, baseUrl }),
    ConfigurationError,
  );
  await assert.rejects(
    () => new GridApiClient({ apiKey: "gp_live_otheraccount", projectKey: PROJECT_KEY, baseUrl }).get("acme", "users"),
    AuthorizationError,
  );
  await assert.rejects(() => client().get("missing", "users"), ProjectNotFoundError);
  await assert.rejects(() => client().getById("acme", "users", "nope"), RecordNotFoundError);
  await assert.rejects(
    () => client().post("acme", "users", { __bad: "x" }),
    ValidationError,
  );
  await assert.rejects(
    () => new GridApiClient({ apiKey: ACCOUNT_KEY, projectKey: PROJECT_KEY, baseUrl: `${baseUrl}/fault/quota` }).get("acme", "users"),
    QuotaExceededError,
  );
});

test("redacts credentials in thrown errors", async () => {
  const api = new GridApiClient({
    apiKey: ACCOUNT_KEY, projectKey: PROJECT_KEY,
    baseUrl: `${baseUrl}/fault/echo`,
    maxRetries: 0,
  });
  await assert.rejects(
    () => api.get("acme", "users"),
    (err) => {
      assert.ok(err instanceof ValidationError);
      assert.ok(!`${err.message} ${err.body}`.includes("gp_live_testkey"));
      return true;
    },
  );
});

test("retries rate limits but never POST; quota is terminal", async () => {
  const flaky = new GridApiClient({
    apiKey: ACCOUNT_KEY, projectKey: PROJECT_KEY,
    baseUrl: `${baseUrl}/fault/rate`,
    maxRetries: 2,
    retryDelayMs: 1,
  });
  assert.equal((await flaky.get("acme", "users")).data.length, 3);

  const noRetry = new GridApiClient({
    apiKey: ACCOUNT_KEY, projectKey: PROJECT_KEY,
    baseUrl: `${baseUrl}/fault/always-rate`,
    maxRetries: 0,
  });
  await assert.rejects(() => noRetry.get("acme", "users"), RateLimitError);

  // POST is never retried: a persistent 429 fails after exactly one call.
  let calls = 0;
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (...args) => {
    if (String(args[0]).includes("/fault/always-rate/api/v1/acme/users")) calls += 1;
    return originalFetch(...args);
  };
  try {
    const write = new GridApiClient({
      apiKey: ACCOUNT_KEY, projectKey: PROJECT_KEY,
      baseUrl: `${baseUrl}/fault/always-rate`,
      maxRetries: 2,
      retryDelayMs: 1,
    });
    await assert.rejects(() => write.post("acme", "users", { name: "RetryProbe" }), RateLimitError);
    assert.equal(calls, 1);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("timeout is enforced and surfaced as TimeoutError", async () => {
  const api = new GridApiClient({
    apiKey: ACCOUNT_KEY, projectKey: PROJECT_KEY,
    baseUrl: `${baseUrl}/fault/timeout`,
    timeoutMs: 60,
    maxRetries: 0,
  });
  await assert.rejects(() => api.get("acme", "users"), TimeoutError);
});

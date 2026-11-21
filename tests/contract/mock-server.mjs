/**
 * In-memory mock of the GridAPI public REST API, implementing exactly the
 * contract in openapi/openapi.yaml. Used by every SDK's contract tests so all
 * languages are exercised against identical behavior.
 *
 * Run: node tests/contract/mock-server.mjs [port]   (default 8787)
 */
import http from "node:http";

const KEY = "gp_live_testkey";
const OTHER_KEY = "gp_live_otheraccount";

const projects = [
  {
    id: "p1", name: "Acme", slug: "acme", role: "OWNER", spreadsheetId: "ss1",
    sheets: [
      { endpoint: "users", name: "Users", sheetName: "Users", primaryKey: "id",
        allowedMethods: ["GET", "POST", "PATCH", "PUT", "DELETE"] },
      { endpoint: "empty", name: "Empty", sheetName: "Empty", primaryKey: "id",
        allowedMethods: ["GET"] },
    ],
  },
  {
    id: "p2", name: "Beta", slug: "beta", role: "MEMBER", spreadsheetId: "ss2",
    sheets: [
      { endpoint: "orders", name: "Orders", sheetName: "Orders", primaryKey: "id",
        allowedMethods: ["GET", "POST", "PATCH", "PUT", "DELETE"] },
    ],
  },
];

const tables = {
  "acme/users": [
    { id: "1", name: "Ann", status: "active", team: "growth" },
    { id: "2", name: "Ben", status: "active", team: "core" },
    { id: "3", name: "Cat", status: "invited", team: "growth" },
  ],
  "acme/empty": [],
  "beta/orders": [{ id: "o1", item: "Widget", qty: "2" }],
};

let rateHits = 0;
let nextId = 100;

function json(res, status, body, headers = {}) {
  const payload = JSON.stringify(body);
  res.writeHead(status, { "content-type": "application/json", ...headers });
  res.end(payload);
}

function readBody(req) {
  return new Promise((resolve) => {
    let raw = "";
    req.on("data", (c) => (raw += c));
    req.on("end", () => {
      try { resolve(raw ? JSON.parse(raw) : undefined); } catch { resolve(undefined); }
    });
  });
}

function parseFilter(value) {
  const out = {};
  if (!value) return out;
  for (const pair of String(value).split(",")) {
    const i = pair.indexOf(":");
    if (i > 0) out[pair.slice(0, i).trim()] = pair.slice(i + 1).trim();
  }
  return out;
}

function authorize(req, url) {
  const header = req.headers.authorization ?? "";
  const key = header.startsWith("Bearer ") ? header.slice(7) : "";
  if (!key) return { error: { error: "Missing API key" }, status: 401 };
  if (key !== KEY && key !== OTHER_KEY) {
    return { error: { error: "Invalid API key", code: "INVALID_API_KEY" }, status: 401 };
  }
  return { key };
}

function checkProject(auth, project) {
  if (project === "missing") {
    return { error: { error: "Project not found", code: "PROJECT_NOT_FOUND" }, status: 404 };
  }
  if (auth.key === OTHER_KEY) {
    return { error: { error: "Project forbidden", code: "PROJECT_FORBIDDEN" }, status: 403 };
  }
  return {};
}

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url ?? "/", "http://localhost");
  let path = url.pathname;
  // Test-only prefix: /fault/<name> injects a failure without altering the contract path.
  const faultMatch = path.match(/^\/fault\/([a-z-]+)(\/.*)$/);
  const fault = faultMatch?.[1] ?? "";
  if (faultMatch) path = faultMatch[2];

  // Discovery reads: authorize but never consume quota.
  if (path === "/api/v1/me" || path === "/api/v1/me/projects" ||
      path === "/api/v1/me/usage" || path.startsWith("/api/v1/me/projects/")) {
    const auth = authorize(req, url);
    if (auth.error) return json(res, auth.status, auth.error);
    if (path === "/api/v1/me") {
      return json(res, 200, { authenticated: true,
        account: { id: auth.key === KEY ? "acct1" : "acct2",
          email: auth.key === KEY ? "owner@example.com" : "other@example.com", scope: "ACCOUNT" },
        subscription: { plan: "startup", totalApiCallsLeft: 900 } });
    }
    if (path === "/api/v1/me/projects") {
      return json(res, 200, { projects: auth.key === KEY ? projects : [] });
    }
    if (path === "/api/v1/me/usage") {
      return json(res, 200, { totalApiCallsUsed: 10, totalApiCallsLeft: 900 });
    }
    const ref = decodeURIComponent(path.split("/")[5] ?? "");
    const project = projects.find((p) => p.id === ref || p.slug === ref);
    if (!project) return json(res, 404, { error: "Project not found", code: "PROJECT_NOT_FOUND" });
    return json(res, 200, { project, dataSource: "google_sheets", endpoints: project.sheets.map((s) => ({
      endpoint: s.endpoint, method: "GET", url: `/api/v1/${project.slug}/${s.endpoint}`,
      primaryKey: s.primaryKey,
      fields: [{ name: "id", type: "string" }, { name: "name", type: "string" }],
      samples: {},
    })) });
  }

  if (path === "/api/v1/capabilities") {
    return json(res, 200, { googleSheets: true, filter: true, pagination: true, sorting: false });
  }

  if (path === "/api/v1/public-docs/acme/users") {
    return json(res, 200, { endpoints: [{ endpoint: "users", fields: [] }] });
  }

  const match = path.match(/^\/api\/v1\/([^/]+)\/([^/]+)(?:\/([^/]+))?$/);
  if (!match) return json(res, 404, { error: "Not found" });

  const [, project, endpoint, id] = match.map((v) => v && decodeURIComponent(v));
  const auth = authorize(req, url);
  if (auth.error) return json(res, auth.status, auth.error);
  const denied = checkProject(auth, project);
  if (denied.error) return json(res, denied.status, denied.error);

  const key = `${project}/${endpoint}`;
  const rows = tables[key];
  if (!rows) return json(res, 404, { error: "Sheet not found", code: "SHEET_NOT_FOUND" });

  // Test-only fault injection, driven by query string — never part of the SDK API.
  if (fault === "always-rate") {
    return json(res, 429, { error: "Too many requests" }, { "retry-after": "0" });
  }
  if (url.searchParams.get("quota") === "1" || fault === "quota") {
    return json(res, 429, { code: "SUBSCRIPTION_LIMIT_REACHED", message: "API call limit exceeded" });
  }
  if (url.searchParams.get("rate") === "1" || fault === "rate") {
    rateHits += 1;
    if (rateHits % 2 === 1) {
      return json(res, 429, { error: "Too many requests" }, { "retry-after": "0" });
    }
  }
  if (url.searchParams.get("timeout") === "1" || fault === "timeout") {
    return; // never respond — exercises SDK timeouts
  }
  if (url.searchParams.get("echoKey") === "1" || fault === "echo") {
    return json(res, 400, { error: `Bad request for ${auth.key}` });
  }

  const method = req.method ?? "GET";

  if (method === "GET" && id === undefined) {
    const filter = parseFilter(url.searchParams.get("filter"));
    let result = rows.filter((row) => Object.entries(filter).every(([k, v]) => row[k] === v));
    const limitRaw = url.searchParams.get("limit");
    const pageRaw = url.searchParams.get("page");
    const limit = limitRaw === null ? 100 : Math.max(1, Math.min(100, Number(limitRaw)));
    const page = pageRaw === null ? undefined : Math.max(1, Number(pageRaw));
    const offset = page !== undefined && limitRaw !== null ? (page - 1) * limit : 0;
    if (offset) result = result.slice(offset, offset + limit);
    else if (limitRaw !== null) result = result.slice(0, limit);
    return json(res, 200, { data: result, meta: { total: rows.filter((row) =>
      Object.entries(filter).every(([k, v]) => row[k] === v)).length, limit, offset } });
  }

  if (method === "GET" && id !== undefined) {
    const row = rows.find((r) => r[endpoint === "orders" ? "id" : "id"] === id);
    if (!row) return json(res, 404, { error: "Record not found", code: "NOT_FOUND" });
    return json(res, 200, row);
  }

  if (method === "POST" && id === undefined) {
    const body = await readBody(req);
    const incoming = Array.isArray(body) ? body : [body];
    if (!body || incoming.length === 0 || incoming.some((v) => typeof v !== "object" || v === null)) {
      return json(res, 500, { error: "Empty payload" });
    }
    if (incoming.some((r) => "__bad" in r)) {
      return json(res, 400, { error: "Unknown column", code: "UNKNOWN_COLUMN" });
    }
    const created = incoming.map((r) => ({ id: `n${nextId++}`, ...r }));
    rows.push(...created);
    return json(res, 200, { inserted: created.length, records: created });
  }

  if ((method === "PATCH" || method === "PUT") && id !== undefined) {
    const body = await readBody(req);
    if (!body || typeof body !== "object" || Array.isArray(body)) {
      return json(res, 400, { error: "Invalid record" });
    }
    if ("__bad" in body) return json(res, 400, { error: "Unknown column", code: "UNKNOWN_COLUMN" });
    const index = rows.findIndex((r) => r.id === id);
    if (index < 0) return json(res, 404, { error: "Record not found", code: "NOT_FOUND" });
    const record = method === "PUT" ? { id, ...body } : { ...rows[index], ...body };
    rows[index] = record;
    return json(res, 200, { updated: 1, record });
  }

  if (method === "DELETE" && id !== undefined) {
    const index = rows.findIndex((r) => r.id === id);
    if (index < 0) return json(res, 404, { error: "Record not found", code: "NOT_FOUND" });
    const [removed] = rows.splice(index, 1);
    return json(res, 200, { deleted: 1, id: removed.id });
  }

  if ((method === "PUT" || method === "DELETE") && id === undefined) {
    const filter = parseFilter(req.headers["x-filter"]);
    if (Object.keys(filter).length === 0) return json(res, 500, { error: "Missing filter" });
    const matched = rows.filter((r) => Object.entries(filter).every(([k, v]) => r[k] === v));
    if (method === "DELETE") {
      matched.forEach((r) => rows.splice(rows.indexOf(r), 1));
      return json(res, 200, { deleted: matched.length, id: Object.values(filter)[0] });
    }
    const body = await readBody(req);
    matched.forEach((r) => Object.assign(r, body));
    return json(res, 200, { updated: matched.length, record: matched[0] ?? {} });
  }

  return json(res, 405, { error: "Method not allowed" });
});

const port = Number(process.argv[2] ?? 8787);
server.listen(port, "127.0.0.1", () => console.log(`mock GridAPI listening on ${port}`));

/**
 * Official TypeScript SDK for the GridAPI public REST API.
 *
 * One account-level API key authenticates every authorized project. The
 * project slug is a routing/authorization target, never a credential.
 * Server-side only: never ship the key in a browser bundle.
 *
 * Contract: openapi/openapi.yaml in the gridapi_sdk monorepo.
 */

export type RecordValue = string | null;

export interface GridRecord {
  [column: string]: RecordValue;
}

export interface ListMeta {
  total: number;
  limit: number;
  offset: number;
  cacheHit?: boolean;
}

export interface ListResponse<T extends GridRecord = GridRecord> {
  data: T[];
  meta: ListMeta;
}

export interface CreateResponse<T extends GridRecord = GridRecord> {
  inserted: number;
  records: T[];
}

export interface UpdateResponse<T extends GridRecord = GridRecord> {
  updated: number;
  record: T;
}

export interface DeleteResponse {
  deleted: number;
  id: string;
}

export interface QueryOptions {
  /** Exact-match filters: either a "col:value,col2:value2" string or an object. */
  filter?: string | Record<string, string>;
  /** 1-based page number. Applied only when `limit` is also supplied. */
  page?: number;
  /** Page size, clamped to 1-100 by the server. */
  limit?: number;
}

export interface GridApiOptions {
  /** Account key (gp_live_...). Defaults to process.env.GRIDAPI_API_KEY. */
  apiKey?: string;
  /** Project key (gapi_live_...). Defaults to process.env.GRIDAPI_PROJECT_KEY. */
  projectKey?: string;
  /** Defaults to process.env.GRIDAPI_BASE_URL or https://api.gridapi.in. */
  baseUrl?: string;
  /** Defaults to process.env.GRIDAPI_TIMEOUT_MS or 30000. */
  timeoutMs?: number;
  /** Retries for transient failures. Default 2. */
  maxRetries?: number;
  /** Base backoff in ms. Default 500. */
  retryDelayMs?: number;
  /** Retry non-idempotent PATCH writes. Default false. POST is never retried. */
  retryWrites?: boolean;
}

export interface SheetInfo {
  endpoint: string;
  name?: string;
  sheetName?: string;
  primaryKey?: string;
  allowedMethods?: string[];
  [key: string]: unknown;
}

export interface ProjectInfo {
  id: string;
  name: string;
  slug: string;
  role: string;
  spreadsheetId?: string | null;
  sheets: SheetInfo[];
  [key: string]: unknown;
}

export interface TableSchema {
  endpoint: string;
  primaryKey?: string;
  fields: Array<{ name: string; type?: string; [key: string]: unknown }>;
}

export interface UsageInfo {
  [key: string]: unknown;
}

export interface AccountInfo {
  authenticated: boolean;
  account?: { id?: string; email?: string; scope?: string; [key: string]: unknown };
  subscription?: Record<string, unknown>;
  [key: string]: unknown;
}

const env = (globalThis as {
  process?: { env?: Record<string, string | undefined> };
}).process?.env ?? {};

/** Remove any credential-looking text before an error reaches a log or user. */
export function redact(value: string): string {
  return value
    .replace(/Bearer\s+\S+/gi, "Bearer [REDACTED]")
    .replace(/(gp_live_|gapi_live_)[A-Za-z0-9_-]+/g, "$1[REDACTED]");
}

export class GridApiError extends Error {
  readonly status?: number;
  readonly code?: string;
  readonly body?: string;
  readonly retryable: boolean;

  constructor(
    message: string,
    options: { status?: number; code?: string; body?: string; retryable?: boolean } = {},
  ) {
    super(redact(message));
    this.name = "GridApiError";
    this.status = options.status;
    this.code = options.code;
    this.body = options.body ? redact(options.body) : undefined;
    this.retryable = options.retryable ?? false;
  }
}

export class ConfigurationError extends GridApiError {
  constructor(message: string, options: ConstructorParameters<typeof GridApiError>[1] = {}) {
    super(message, options);
    this.name = "ConfigurationError";
  }
}

export class AuthenticationError extends GridApiError {
  constructor(message: string, options: ConstructorParameters<typeof GridApiError>[1] = {}) {
    super(message, options);
    this.name = "AuthenticationError";
  }
}
export class AuthorizationError extends GridApiError {
  constructor(message: string, options: ConstructorParameters<typeof GridApiError>[1] = {}) {
    super(message, options);
    this.name = "AuthorizationError";
  }
}
export class ProjectNotFoundError extends GridApiError {
  constructor(message: string, options: ConstructorParameters<typeof GridApiError>[1] = {}) {
    super(message, options);
    this.name = "ProjectNotFoundError";
  }
}
export class TableNotFoundError extends GridApiError {
  constructor(message: string, options: ConstructorParameters<typeof GridApiError>[1] = {}) {
    super(message, options);
    this.name = "TableNotFoundError";
  }
}
export class RecordNotFoundError extends GridApiError {
  constructor(message: string, options: ConstructorParameters<typeof GridApiError>[1] = {}) {
    super(message, options);
    this.name = "RecordNotFoundError";
  }
}
export class ValidationError extends GridApiError {
  constructor(message: string, options: ConstructorParameters<typeof GridApiError>[1] = {}) {
    super(message, options);
    this.name = "ValidationError";
  }
}
export class RateLimitError extends GridApiError {
  constructor(message: string, options: ConstructorParameters<typeof GridApiError>[1] = {}) {
    super(message, { ...options, retryable: true });
    this.name = "RateLimitError";
  }
}
export class QuotaExceededError extends GridApiError {
  constructor(message: string, options: ConstructorParameters<typeof GridApiError>[1] = {}) {
    super(message, options);
    this.name = "QuotaExceededError";
  }
}
export class UpstreamProviderError extends GridApiError {
  constructor(message: string, options: ConstructorParameters<typeof GridApiError>[1] = {}) {
    super(message, { ...options, retryable: true });
    this.name = "UpstreamProviderError";
  }
}
export class NetworkError extends GridApiError {
  constructor(message: string, options: ConstructorParameters<typeof GridApiError>[1] = {}) {
    super(message, { ...options, retryable: true });
    this.name = "NetworkError";
  }
}
export class TimeoutError extends GridApiError {
  constructor(message: string, options: ConstructorParameters<typeof GridApiError>[1] = {}) {
    super(message, { ...options, retryable: true });
    this.name = "TimeoutError";
  }
}

function mapHttpError(status: number, code: string | undefined, message: string, body: string): GridApiError {
  const opts = { status, code, body };
  if (status === 401) return new AuthenticationError(message, opts);
  if (status === 403) return new AuthorizationError(message, opts);
  if (status === 404) {
    if (code === "SHEET_NOT_FOUND" || /sheet|table/i.test(message)) return new TableNotFoundError(message, opts);
    if (code === "NOT_FOUND" || /record/i.test(message)) return new RecordNotFoundError(message, opts);
    return new ProjectNotFoundError(message, opts);
  }
  if (status === 400 || status === 422) return new ValidationError(message, opts);
  if (status === 429) {
    return code === "SUBSCRIPTION_LIMIT_REACHED"
      ? new QuotaExceededError(message, opts)
      : new RateLimitError(message, opts);
  }
  if (status >= 502) return new UpstreamProviderError(message, opts);
  return new GridApiError(message, { ...opts, retryable: status >= 500 });
}

function parseBody(text: string): unknown {
  if (!text) return undefined;
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return text;
  }
}

function asMessage(parsed: unknown, fallback: string): { message: string; code?: string } {
  if (parsed && typeof parsed === "object") {
    const obj = parsed as Record<string, unknown>;
    const code = typeof obj.code === "string" ? obj.code : undefined;
    const message = typeof obj.error === "string" ? obj.error
      : typeof obj.message === "string" ? obj.message : fallback;
    return { message, code };
  }
  return { message: typeof parsed === "string" && parsed ? parsed : fallback };
}

function retryDelayMs(retryAfter: string | null, base: number, attempt: number): number {
  if (retryAfter) {
    const seconds = Number(retryAfter);
    if (Number.isFinite(seconds)) return Math.max(0, seconds * 1000);
    const at = Date.parse(retryAfter);
    if (!Number.isNaN(at)) return Math.max(0, at - Date.now());
  }
  const jitter = Math.floor(Math.random() * (base / 2));
  return Math.min(30_000, base * 2 ** attempt) + jitter;
}

const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

const ACCOUNT_PREFIX = "gp_live_";
const PROJECT_PREFIX = "gapi_live_";

function requiredKey(
  value: string | undefined,
  prefix: string,
  envName: string,
  optionName: string,
): string {
  if (!value) throw new ConfigurationError(`GridAPI: missing ${envName}. Set it or pass ${optionName}.`);
  if (!value.startsWith(prefix) || value.length === prefix.length) {
    throw new ConfigurationError(`GridAPI: ${envName} must be a ${prefix}... key.`);
  }
  return value;
}

export class GridApiClient {
  private readonly apiKey: string;
  private readonly projectKey: string;
  private accountVerification?: Promise<void>;
  private readonly verifiedProjects = new Set<string>();
  private readonly baseUrl: string;
  private readonly timeoutMs: number;
  private readonly maxRetries: number;
  private readonly retryDelayMs: number;
  private readonly retryWrites: boolean;

  constructor(options: GridApiOptions = {}) {
    this.apiKey = requiredKey(
      options.apiKey ?? env.GRIDAPI_API_KEY, ACCOUNT_PREFIX, "GRIDAPI_API_KEY", "apiKey",
    );
    // Required locally as a scope guard; the public REST API has no project-key
    // header, so only the account key is sent and scope is asserted via /me/projects.
    this.projectKey = requiredKey(
      options.projectKey ?? env.GRIDAPI_PROJECT_KEY,
      PROJECT_PREFIX, "GRIDAPI_PROJECT_KEY", "projectKey",
    );
    this.baseUrl = (options.baseUrl ?? env.GRIDAPI_BASE_URL ?? "https://api.gridapi.in")
      .replace(/\/+$/, "");
    this.timeoutMs = options.timeoutMs ?? (Number(env.GRIDAPI_TIMEOUT_MS) || 30_000);
    this.maxRetries = options.maxRetries ?? 2;
    this.retryDelayMs = options.retryDelayMs ?? 500;
    this.retryWrites = options.retryWrites ?? false;
  }

  /**
   * Confirm the account key is live and the project is in this account's
   * authorized projects. Runs once per account and once per project, and is
   * called automatically by every data method.
   */
  async verify(project?: string): Promise<void> {
    this.accountVerification ??= (async () => {
      const me = await this.getMe();
      if (!me.authenticated) {
        throw new AuthenticationError("GridAPI: account key is not authenticated.");
      }
    })();
    await this.accountVerification;
    if (project && !this.verifiedProjects.has(project)) {
      const projects = await this.listProjects();
      const allowed = projects.some(
        (p) => p.slug === project || p.id === project || p.name === project,
      );
      if (!allowed) {
        if (projects.length === 0) {
          throw new AuthorizationError(
            "GridAPI: no projects are authorized for this account and project key.",
          );
        }
        throw new ProjectNotFoundError(`Project not found: ${project}`);
      }
      this.verifiedProjects.add(project);
    }
  }

  private dataPath(project: string, table: string, id?: string): string {
    const parts = ["api", "v1", encodeURIComponent(project), encodeURIComponent(table)];
    if (id !== undefined && id !== null) parts.push(encodeURIComponent(String(id)));
    return `/${parts.join("/")}`;
  }

  private queryString(params?: QueryOptions): string {
    if (!params) return "";
    const q = new URLSearchParams();
    if (params.filter !== undefined) {
      q.set("filter", typeof params.filter === "string"
        ? params.filter
        : Object.entries(params.filter).map(([k, v]) => `${k}:${v}`).join(","));
    }
    if (params.page !== undefined) q.set("page", String(params.page));
    if (params.limit !== undefined) q.set("limit", String(params.limit));
    const s = q.toString();
    return s ? `?${s}` : "";
  }

  private async request(method: string, path: string, body?: unknown): Promise<unknown> {
    const url = `${this.baseUrl}${path}`;
    const serialised = body === undefined ? undefined : JSON.stringify(body);
    const safe = method === "GET" || method === "DELETE" || method === "PUT" ||
      (method === "PATCH" && this.retryWrites);

    for (let attempt = 0; ; attempt++) {
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), this.timeoutMs);
      try {
        const response = await fetch(url, {
          method,
          headers: {
            Authorization: `Bearer ${this.apiKey}`,
            "Content-Type": "application/json",
            Accept: "application/json",
          },
          body: serialised,
          signal: controller.signal,
        });
        clearTimeout(timer);
        const text = await response.text();
        if (!response.ok) {
          const parsed = parseBody(text);
          const { message, code } = asMessage(parsed, `GridAPI request failed with status ${response.status}`);
          const error = mapHttpError(response.status, code, message, text);
          const retryAfter = response.headers.get("retry-after");
          if (safe && attempt < this.maxRetries && error.retryable) {
            await sleep(retryDelayMs(retryAfter, this.retryDelayMs, attempt));
            continue;
          }
          throw error;
        }
        return parseBody(text);
      } catch (error) {
        clearTimeout(timer);
        if (error instanceof GridApiError) throw error;
        const timedOut = controller.signal.aborted;
        const wrapped = timedOut
          ? new TimeoutError("GridAPI request timed out")
          : new NetworkError(`GridAPI network error: ${error instanceof Error ? error.message : "unknown"}`);
        if (safe && attempt < this.maxRetries) {
          await sleep(retryDelayMs(null, this.retryDelayMs, attempt));
          continue;
        }
        throw wrapped;
      }
    }
  }

  async get<T extends GridRecord = GridRecord>(project: string, table: string, params?: QueryOptions): Promise<ListResponse<T>> {
    await this.verify(project);
    return this.request("GET", this.dataPath(project, table) + this.queryString(params)) as Promise<ListResponse<T>>;
  }

  async getById<T extends GridRecord = GridRecord>(project: string, table: string, id: string | number): Promise<T> {
    await this.verify(project);
    return this.request("GET", this.dataPath(project, table, String(id))) as Promise<T>;
  }

  async post<T extends GridRecord = GridRecord>(project: string, table: string, record: GridRecord | GridRecord[]): Promise<CreateResponse<T>> {
    await this.verify(project);
    return this.request("POST", this.dataPath(project, table), record) as Promise<CreateResponse<T>>;
  }

  async patch<T extends GridRecord = GridRecord>(project: string, table: string, id: string | number, changes: GridRecord): Promise<UpdateResponse<T>> {
    await this.verify(project);
    return this.request("PATCH", this.dataPath(project, table, String(id)), changes) as Promise<UpdateResponse<T>>;
  }

  async put<T extends GridRecord = GridRecord>(project: string, table: string, id: string | number, record: GridRecord): Promise<UpdateResponse<T>> {
    await this.verify(project);
    return this.request("PUT", this.dataPath(project, table, String(id)), record) as Promise<UpdateResponse<T>>;
  }

  async delete(project: string, table: string, id: string | number): Promise<DeleteResponse> {
    await this.verify(project);
    return this.request("DELETE", this.dataPath(project, table, String(id))) as Promise<DeleteResponse>;
  }

  query<T extends GridRecord = GridRecord>(project: string, table: string, params: QueryOptions = {}): Promise<ListResponse<T>> {
    return this.get<T>(project, table, params);
  }

  /** Uses the list response's meta.total with limit=1; there is no count endpoint. */
  async count(project: string, table: string, filter?: QueryOptions["filter"]): Promise<number> {
    const result = await this.get(project, table, { filter, limit: 1 });
    return result.meta.total;
  }

  async listProjects(): Promise<ProjectInfo[]> {
    const result = await this.request("GET", "/api/v1/me/projects") as { projects?: ProjectInfo[] };
    return result.projects ?? [];
  }

  async listTables(project: string): Promise<SheetInfo[]> {
    const projects = await this.listProjects();
    const found = projects.find((p) => p.slug === project || p.id === project || p.name === project);
    if (!found) throw new ProjectNotFoundError(`Project not found: ${project}`);
    return found.sheets ?? [];
  }

  async getSchema(project: string, table: string): Promise<TableSchema> {
    const docs = await this.request("GET", `/api/v1/me/projects/${encodeURIComponent(project)}/docs`) as {
      endpoints?: Array<Record<string, unknown>>;
    };
    const endpoint = (docs.endpoints ?? []).find(
      (e) => String(e.endpoint ?? "").toLowerCase() === table.toLowerCase(),
    );
    if (!endpoint) throw new TableNotFoundError(`Table not found: ${table}`);
    return endpoint as unknown as TableSchema;
  }

  getMe(): Promise<AccountInfo> {
    return this.request("GET", "/api/v1/me") as Promise<AccountInfo>;
  }

  getUsage(): Promise<UsageInfo> {
    return this.request("GET", "/api/v1/me/usage") as Promise<UsageInfo>;
  }
}

/** Alias matching the example in the public documentation. */
export { GridApiClient as GridAPI };
export default GridApiClient;

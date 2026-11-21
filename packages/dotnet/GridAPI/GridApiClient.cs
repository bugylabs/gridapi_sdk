/**
 * Official .NET SDK for the GridAPI public REST API.
 *
 * One account-level API key authenticates every authorized project. The project
 * slug is a routing/authorization target, never a credential.
 * Server-side only: never ship the key in a browser or client application.
 *
 * Contract: openapi/openapi.yaml in the gridapi_sdk monorepo.
 */
using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Text.RegularExpressions;
using System.Threading;
using System.Threading.Tasks;

namespace GridAPI;

/** A single row: column name to string value (or null). */
public class GridRecord : Dictionary<string, string?>
{
}

public sealed class ListMeta
{
    [JsonPropertyName("total")] public int Total { get; set; }
    [JsonPropertyName("limit")] public int Limit { get; set; }
    [JsonPropertyName("offset")] public int Offset { get; set; }
    [JsonPropertyName("cacheHit")] public bool? CacheHit { get; set; }
}

public sealed class ListResponse<T> where T : GridRecord, new()
{
    [JsonPropertyName("data")] public List<T> Data { get; set; } = new();
    [JsonPropertyName("meta")] public ListMeta Meta { get; set; } = new();
}

public sealed class CreateResponse<T> where T : GridRecord, new()
{
    [JsonPropertyName("inserted")] public int Inserted { get; set; }
    [JsonPropertyName("records")] public List<T> Records { get; set; } = new();
}

public sealed class UpdateResponse<T> where T : GridRecord, new()
{
    [JsonPropertyName("updated")] public int Updated { get; set; }
    [JsonPropertyName("record")] public T? Record { get; set; }
}

public sealed class DeleteResponse
{
    [JsonPropertyName("deleted")] public int Deleted { get; set; }
    [JsonPropertyName("id")] public string? Id { get; set; }
}

/** Exact-match filter plus 1-based page and page size (server clamps 1-100). */
public sealed class QueryOptions
{
    public QueryOptions()
    {
    }

    public QueryOptions(IDictionary<string, string> filter)
    {
        Filter = string.Join(",", filter.Select(kv => $"{kv.Key}:{kv.Value}"));
    }

    /** Either "col:value,col2:value2" or a value built from a filter dictionary. */
    public string? Filter { get; set; }
    public int? Page { get; set; }
    public int? Limit { get; set; }
}

public sealed class GridApiOptions
{
    /** Defaults to GRIDAPI_API_KEY (required). */
    public string? ApiKey { get; set; }
    /** Defaults to GRIDAPI_BASE_URL or https://api.gridapi.in. */
    public string? BaseUrl { get; set; }
    /** Defaults to GRIDAPI_TIMEOUT_MS or 30000. */
    public int? TimeoutMs { get; set; }
    /** Transient-failure retries for safe methods. Default 2. */
    public int? MaxRetries { get; set; }
    /** Base backoff in ms. Default 500. */
    public int? RetryDelayMs { get; set; }
    /** Retry non-idempotent PATCH writes. Default false. POST is never retried. */
    public bool? RetryWrites { get; set; }
}

public sealed class SheetInfo
{
    [JsonPropertyName("endpoint")] public string Endpoint { get; set; } = "";
    [JsonPropertyName("name")] public string? Name { get; set; }
    [JsonPropertyName("sheetName")] public string? SheetName { get; set; }
    [JsonPropertyName("primaryKey")] public string? PrimaryKey { get; set; }
    [JsonPropertyName("allowedMethods")] public List<string>? AllowedMethods { get; set; }

    [JsonExtensionData] public Dictionary<string, JsonElement>? Extra { get; set; }
}

public sealed class ProjectInfo
{
    [JsonPropertyName("id")] public string Id { get; set; } = "";
    [JsonPropertyName("name")] public string Name { get; set; } = "";
    [JsonPropertyName("slug")] public string Slug { get; set; } = "";
    [JsonPropertyName("role")] public string Role { get; set; } = "";
    [JsonPropertyName("spreadsheetId")] public string? SpreadsheetId { get; set; }
    [JsonPropertyName("sheets")] public List<SheetInfo> Sheets { get; set; } = new();

    [JsonExtensionData] public Dictionary<string, JsonElement>? Extra { get; set; }
}

public sealed class TableField
{
    [JsonPropertyName("name")] public string Name { get; set; } = "";
    [JsonPropertyName("type")] public string? Type { get; set; }

    [JsonExtensionData] public Dictionary<string, JsonElement>? Extra { get; set; }
}

public sealed class TableSchema
{
    [JsonPropertyName("endpoint")] public string Endpoint { get; set; } = "";
    [JsonPropertyName("primaryKey")] public string? PrimaryKey { get; set; }
    [JsonPropertyName("fields")] public List<TableField> Fields { get; set; } = new();

    [JsonExtensionData] public Dictionary<string, JsonElement>? Extra { get; set; }
}

public sealed class AccountDetails
{
    [JsonPropertyName("id")] public string? Id { get; set; }
    [JsonPropertyName("email")] public string? Email { get; set; }
    [JsonPropertyName("scope")] public string? Scope { get; set; }

    [JsonExtensionData] public Dictionary<string, JsonElement>? Extra { get; set; }
}

public sealed class AccountInfo
{
    [JsonPropertyName("authenticated")] public bool Authenticated { get; set; }
    [JsonPropertyName("account")] public AccountDetails? Account { get; set; }
    [JsonPropertyName("subscription")] public Dictionary<string, JsonElement>? Subscription { get; set; }

    [JsonExtensionData] public Dictionary<string, JsonElement>? Extra { get; set; }
}

public sealed class UsageInfo : Dictionary<string, JsonElement>
{
}

/** Remove any credential-looking text before an error reaches a log or user. */
public static class GridApiRedaction
{
    private static readonly Regex BearerRegex =
        new(@"Bearer\s+\S+", RegexOptions.IgnoreCase | RegexOptions.Compiled);
    private static readonly Regex KeyRegex =
        new(@"(gp_live_|gapi_live_)[A-Za-z0-9_-]+", RegexOptions.Compiled);

    public static string Redact(string value)
    {
        if (string.IsNullOrEmpty(value)) return value;
        return KeyRegex.Replace(BearerRegex.Replace(value, "Bearer [REDACTED]"), "$1[REDACTED]");
    }
}

public class GridApiError : Exception
{
    public GridApiError(
        string message,
        int? status = null,
        string? code = null,
        string? body = null,
        bool retryable = false)
        : base(GridApiRedaction.Redact(message))
    {
        Status = status;
        Code = code;
        Body = body is null ? null : GridApiRedaction.Redact(body);
        Retryable = retryable;
    }

    public int? Status { get; }
    public string? Code { get; }
    public string? Body { get; }
    public bool Retryable { get; }
}

public sealed class AuthenticationError : GridApiError
{
    public AuthenticationError(string message, int? status = null, string? code = null, string? body = null, bool retryable = false)
        : base(message, status, code, body, retryable) { }
}

public sealed class AuthorizationError : GridApiError
{
    public AuthorizationError(string message, int? status = null, string? code = null, string? body = null, bool retryable = false)
        : base(message, status, code, body, retryable) { }
}

public sealed class ProjectNotFoundError : GridApiError
{
    public ProjectNotFoundError(string message, int? status = null, string? code = null, string? body = null, bool retryable = false)
        : base(message, status, code, body, retryable) { }
}

public sealed class TableNotFoundError : GridApiError
{
    public TableNotFoundError(string message, int? status = null, string? code = null, string? body = null, bool retryable = false)
        : base(message, status, code, body, retryable) { }
}

public sealed class RecordNotFoundError : GridApiError
{
    public RecordNotFoundError(string message, int? status = null, string? code = null, string? body = null, bool retryable = false)
        : base(message, status, code, body, retryable) { }
}

public sealed class ValidationError : GridApiError
{
    public ValidationError(string message, int? status = null, string? code = null, string? body = null, bool retryable = false)
        : base(message, status, code, body, retryable) { }
}

public sealed class RateLimitError : GridApiError
{
    public RateLimitError(string message, int? status = null, string? code = null, string? body = null, bool retryable = true)
        : base(message, status, code, body, retryable) { }
}

public sealed class QuotaExceededError : GridApiError
{
    public QuotaExceededError(string message, int? status = null, string? code = null, string? body = null, bool retryable = false)
        : base(message, status, code, body, retryable) { }
}

public sealed class UpstreamProviderError : GridApiError
{
    public UpstreamProviderError(string message, int? status = null, string? code = null, string? body = null, bool retryable = true)
        : base(message, status, code, body, retryable) { }
}

public sealed class NetworkError : GridApiError
{
    public NetworkError(string message, int? status = null, string? code = null, string? body = null, bool retryable = true)
        : base(message, status, code, body, retryable) { }
}

public sealed class TimeoutError : GridApiError
{
    public TimeoutError(string message, int? status = null, string? code = null, string? body = null, bool retryable = true)
        : base(message, status, code, body, retryable) { }
}

public sealed class GridApiClient : IDisposable
{
    private readonly string _apiKey;
    private readonly string _baseUrl;
    private readonly int _timeoutMs;
    private readonly int _maxRetries;
    private readonly int _retryDelayMs;
    private readonly bool _retryWrites;
    private readonly HttpClient _http;
    private readonly JsonSerializerOptions _json;

    public GridApiClient(GridApiOptions? options = null)
    {
        var key = options?.ApiKey ?? Environment.GetEnvironmentVariable("GRIDAPI_API_KEY");
        if (string.IsNullOrEmpty(key))
            throw new AuthenticationError("GridAPI: missing API key. Set GRIDAPI_API_KEY or pass ApiKey in options.");

        _apiKey = key;
        _baseUrl = (options?.BaseUrl ?? Environment.GetEnvironmentVariable("GRIDAPI_BASE_URL")
                    ?? "https://api.gridapi.in").TrimEnd('/');
        var timeoutRaw = Environment.GetEnvironmentVariable("GRIDAPI_TIMEOUT_MS");
        _timeoutMs = options?.TimeoutMs
                     ?? (int.TryParse(timeoutRaw, out var parsed) && parsed > 0 ? parsed : 30_000);
        _maxRetries = options?.MaxRetries ?? 2;
        _retryDelayMs = options?.RetryDelayMs ?? 500;
        _retryWrites = options?.RetryWrites ?? false;

        // Per-attempt cancellation drives timeouts so they can be distinguished from caller cancellation.
        _http = new HttpClient { Timeout = System.Threading.Timeout.InfiniteTimeSpan };
        _json = new JsonSerializerOptions { PropertyNameCaseInsensitive = true };
    }

    public void Dispose() => _http.Dispose();

    private static string DataPath(string project, string table, string? id = null)
    {
        var p = Uri.EscapeDataString(project);
        var t = Uri.EscapeDataString(table);
        return id is null
            ? $"/api/v1/{p}/{t}"
            : $"/api/v1/{p}/{t}/{Uri.EscapeDataString(id)}";
    }

    private static string QueryString(QueryOptions? options)
    {
        if (options is null) return "";
        var parts = new List<string>();
        if (options.Filter is not null) parts.Add("filter=" + Uri.EscapeDataString(options.Filter));
        if (options.Page is not null) parts.Add("page=" + options.Page.Value);
        if (options.Limit is not null) parts.Add("limit=" + options.Limit.Value);
        return parts.Count == 0 ? "" : "?" + string.Join("&", parts);
    }

    private static GridApiError BuildError(int status, string text)
    {
        string? code = null;
        var message = $"GridAPI request failed with status {status}";
        if (!string.IsNullOrEmpty(text))
        {
            try
            {
                using var doc = JsonDocument.Parse(text);
                if (doc.RootElement.ValueKind == JsonValueKind.Object)
                {
                    if (doc.RootElement.TryGetProperty("code", out var codeEl) && codeEl.ValueKind == JsonValueKind.String)
                        code = codeEl.GetString();
                    if (doc.RootElement.TryGetProperty("error", out var errEl) && errEl.ValueKind == JsonValueKind.String)
                        message = errEl.GetString() ?? message;
                    else if (doc.RootElement.TryGetProperty("message", out var msgEl) && msgEl.ValueKind == JsonValueKind.String)
                        message = msgEl.GetString() ?? message;
                }
                else if (doc.RootElement.ValueKind == JsonValueKind.String)
                {
                    message = doc.RootElement.GetString() ?? message;
                }
            }
            catch (JsonException)
            {
                if (text.Length > 0) message = text;
            }
        }

        bool HasCode(string expected) => string.Equals(code, expected, StringComparison.OrdinalIgnoreCase);
        bool Says(string word) => message.Contains(word, StringComparison.OrdinalIgnoreCase);

        if (status == 401) return new AuthenticationError(message, status, code, text);
        if (status == 403) return new AuthorizationError(message, status, code, text);
        if (status == 404)
        {
            if (HasCode("SHEET_NOT_FOUND") || Says("sheet") || Says("table")) return new TableNotFoundError(message, status, code, text);
            if (HasCode("NOT_FOUND") || Says("record")) return new RecordNotFoundError(message, status, code, text);
            return new ProjectNotFoundError(message, status, code, text);
        }
        if (status == 400 || (status >= 400 && status <= 422)) return new ValidationError(message, status, code, text);
        if (status == 429)
            return HasCode("SUBSCRIPTION_LIMIT_REACHED")
                ? new QuotaExceededError(message, status, code, text)
                : new RateLimitError(message, status, code, text);
        if (status >= 502) return new UpstreamProviderError(message, status, code, text);
        return new GridApiError(message, status, code, text, retryable: status >= 500);
    }

    private static TimeSpan? RetryAfter(HttpResponseMessage response)
    {
        var value = response.Headers.RetryAfter;
        if (value is null) return null;
        if (value.Delta is { } delta) return delta;
        if (value.Date is { } date)
            return TimeSpan.FromMilliseconds(Math.Max(0, (date - DateTimeOffset.UtcNow).TotalMilliseconds));
        return null;
    }

    private TimeSpan Backoff(TimeSpan? retryAfter, int attempt)
    {
        if (retryAfter is { } specified) return specified;
        var exponential = Math.Min(30_000L, (long)_retryDelayMs * (1L << Math.Min(attempt, 20)));
        var jitter = Random.Shared.Next(0, Math.Max(1, _retryDelayMs / 2));
        return TimeSpan.FromMilliseconds(exponential + jitter);
    }

    private async Task<TResult> RequestAsync<TResult>(HttpMethod method, string path, object? body, CancellationToken cancellationToken)
    {
        var safe = method == HttpMethod.Get || method == HttpMethod.Delete || method == HttpMethod.Put
                   || (method == HttpMethod.Patch && _retryWrites);
        var serialised = body is null ? null : JsonSerializer.Serialize(body, _json);

        for (var attempt = 0; ; attempt++)
        {
            using var attemptCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
            attemptCts.CancelAfter(_timeoutMs);
            try
            {
                using var request = new HttpRequestMessage(method, new Uri(_baseUrl + path));
                request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", _apiKey);
                request.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
                if (serialised is not null)
                    request.Content = new StringContent(serialised, Encoding.UTF8, "application/json");

                using var response = await _http.SendAsync(request, attemptCts.Token).ConfigureAwait(false);
                var text = await response.Content.ReadAsStringAsync(attemptCts.Token).ConfigureAwait(false);
                if (!response.IsSuccessStatusCode)
                {
                    var error = BuildError((int)response.StatusCode, text);
                    if (safe && attempt < _maxRetries && error.Retryable)
                    {
                        await Task.Delay(Backoff(RetryAfter(response), attempt), cancellationToken).ConfigureAwait(false);
                        continue;
                    }
                    throw error;
                }
                if (string.IsNullOrEmpty(text)) return default!;
                return JsonSerializer.Deserialize<TResult>(text, _json)!;
            }
            catch (GridApiError)
            {
                throw;
            }
            catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested)
            {
                var error = new TimeoutError("GridAPI request timed out");
                if (safe && attempt < _maxRetries)
                {
                    await Task.Delay(Backoff(null, attempt), cancellationToken).ConfigureAwait(false);
                    continue;
                }
                throw error;
            }
            catch (Exception ex) when (ex is HttpRequestException or IOException)
            {
                if (safe && attempt < _maxRetries)
                {
                    await Task.Delay(Backoff(null, attempt), cancellationToken).ConfigureAwait(false);
                    continue;
                }
                throw new NetworkError($"GridAPI network error: {ex.Message}");
            }
        }
    }

    public Task<ListResponse<GridRecord>> GetAsync(string project, string table, QueryOptions? options = null, CancellationToken cancellationToken = default)
        => GetAsync<GridRecord>(project, table, options, cancellationToken);

    public Task<ListResponse<T>> GetAsync<T>(string project, string table, QueryOptions? options = null, CancellationToken cancellationToken = default)
        where T : GridRecord, new()
        => RequestAsync<ListResponse<T>>(HttpMethod.Get, DataPath(project, table) + QueryString(options), null, cancellationToken);

    public Task<GridRecord> GetByIdAsync(string project, string table, string id, CancellationToken cancellationToken = default)
        => GetByIdAsync<GridRecord>(project, table, id, cancellationToken);

    public Task<T> GetByIdAsync<T>(string project, string table, string id, CancellationToken cancellationToken = default)
        where T : GridRecord, new()
        => RequestAsync<T>(HttpMethod.Get, DataPath(project, table, id), null, cancellationToken);

    public Task<CreateResponse<GridRecord>> PostAsync(string project, string table, GridRecord record, CancellationToken cancellationToken = default)
        => PostAsync<GridRecord>(project, table, record, cancellationToken);

    public Task<CreateResponse<GridRecord>> PostAsync(string project, string table, IEnumerable<GridRecord> records, CancellationToken cancellationToken = default)
        => PostAsync<GridRecord>(project, table, records, cancellationToken);

    public Task<CreateResponse<T>> PostAsync<T>(string project, string table, GridRecord record, CancellationToken cancellationToken = default)
        where T : GridRecord, new()
        => RequestAsync<CreateResponse<T>>(HttpMethod.Post, DataPath(project, table), record, cancellationToken);

    public Task<CreateResponse<T>> PostAsync<T>(string project, string table, IEnumerable<GridRecord> records, CancellationToken cancellationToken = default)
        where T : GridRecord, new()
        => RequestAsync<CreateResponse<T>>(HttpMethod.Post, DataPath(project, table), records, cancellationToken);

    public Task<UpdateResponse<GridRecord>> PatchAsync(string project, string table, string id, GridRecord changes, CancellationToken cancellationToken = default)
        => PatchAsync<GridRecord>(project, table, id, changes, cancellationToken);

    public Task<UpdateResponse<T>> PatchAsync<T>(string project, string table, string id, GridRecord changes, CancellationToken cancellationToken = default)
        where T : GridRecord, new()
        => RequestAsync<UpdateResponse<T>>(HttpMethod.Patch, DataPath(project, table, id), changes, cancellationToken);

    public Task<UpdateResponse<GridRecord>> PutAsync(string project, string table, string id, GridRecord record, CancellationToken cancellationToken = default)
        => PutAsync<GridRecord>(project, table, id, record, cancellationToken);

    public Task<UpdateResponse<T>> PutAsync<T>(string project, string table, string id, GridRecord record, CancellationToken cancellationToken = default)
        where T : GridRecord, new()
        => RequestAsync<UpdateResponse<T>>(HttpMethod.Put, DataPath(project, table, id), record, cancellationToken);

    public Task<DeleteResponse> DeleteAsync(string project, string table, string id, CancellationToken cancellationToken = default)
        => RequestAsync<DeleteResponse>(HttpMethod.Delete, DataPath(project, table, id), null, cancellationToken);

    public Task<ListResponse<GridRecord>> QueryAsync(string project, string table, QueryOptions? options = null, CancellationToken cancellationToken = default)
        => GetAsync(project, table, options, cancellationToken);

    public Task<ListResponse<T>> QueryAsync<T>(string project, string table, QueryOptions? options = null, CancellationToken cancellationToken = default)
        where T : GridRecord, new()
        => GetAsync<T>(project, table, options, cancellationToken);

    /** Uses the list response's meta.total with limit=1; there is no count endpoint. */
    public async Task<int> CountAsync(string project, string table, string? filter = null, CancellationToken cancellationToken = default)
    {
        var result = await GetAsync(project, table, new QueryOptions { Filter = filter, Limit = 1 }, cancellationToken).ConfigureAwait(false);
        return result.Meta.Total;
    }

    private sealed class ProjectsResponse
    {
        [JsonPropertyName("projects")] public List<ProjectInfo> Projects { get; set; } = new();
    }

    private sealed class DocsResponse
    {
        [JsonPropertyName("endpoints")] public List<TableSchema> Endpoints { get; set; } = new();
    }

    public async Task<IReadOnlyList<ProjectInfo>> ListProjectsAsync(CancellationToken cancellationToken = default)
    {
        var result = await RequestAsync<ProjectsResponse>(HttpMethod.Get, "/api/v1/me/projects", null, cancellationToken).ConfigureAwait(false);
        return result.Projects;
    }

    public async Task<IReadOnlyList<SheetInfo>> ListTablesAsync(string project, CancellationToken cancellationToken = default)
    {
        var projects = await ListProjectsAsync(cancellationToken).ConfigureAwait(false);
        var found = projects.FirstOrDefault(p => p.Slug == project || p.Id == project || p.Name == project);
        if (found is null) throw new ProjectNotFoundError($"Project not found: {project}");
        return found.Sheets;
    }

    public async Task<TableSchema> GetSchemaAsync(string project, string table, CancellationToken cancellationToken = default)
    {
        var refPart = Uri.EscapeDataString(project);
        var docs = await RequestAsync<DocsResponse>(HttpMethod.Get, $"/api/v1/me/projects/{refPart}/docs", null, cancellationToken).ConfigureAwait(false);
        var endpoint = docs.Endpoints.FirstOrDefault(e => string.Equals(e.Endpoint, table, StringComparison.OrdinalIgnoreCase));
        if (endpoint is null) throw new TableNotFoundError($"Table not found: {table}");
        return endpoint;
    }

    public Task<AccountInfo> GetMeAsync(CancellationToken cancellationToken = default)
        => RequestAsync<AccountInfo>(HttpMethod.Get, "/api/v1/me", null, cancellationToken);

    public Task<UsageInfo> GetUsageAsync(CancellationToken cancellationToken = default)
        => RequestAsync<UsageInfo>(HttpMethod.Get, "/api/v1/me/usage", null, cancellationToken);
}

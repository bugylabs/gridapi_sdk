package io.gridapi;

import java.io.IOException;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.net.http.HttpTimeoutException;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.time.Instant;
import java.time.ZonedDateTime;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.concurrent.ThreadLocalRandom;
import java.util.regex.Matcher;

/**
 * Official Java SDK for the GridAPI public REST API.
 *
 * One account-level key authenticates every authorized project; the project
 * slug is a routing target, never a credential. Server-side only: never ship
 * the key in a browser or mobile bundle.
 *
 * Reads GRIDAPI_API_KEY (required), GRIDAPI_BASE_URL (default
 * https://api.gridapi.in) and GRIDAPI_TIMEOUT_MS (default 30000), each
 * overridable through {@link #builder()}.
 */
public final class GridApiClient {
    private static final String DEFAULT_BASE_URL = "https://api.gridapi.in";
    private static final int DEFAULT_TIMEOUT_MS = 30_000;

    private final String apiKey;
    private final String baseUrl;
    private final int timeoutMs;
    private final int maxRetries;
    private final int retryDelayMs;
    private final boolean retryWrites;
    private final HttpClient http;

    private GridApiClient(Builder b) {
        String key = b.apiKey != null ? b.apiKey : env("GRIDAPI_API_KEY");
        if (key == null || key.isBlank()) {
            throw new GridApiError.AuthenticationError(
                "GridAPI: missing API key. Set GRIDAPI_API_KEY or pass apiKey to the builder.");
        }
        this.apiKey = key;

        String url = b.baseUrl != null ? b.baseUrl : env("GRIDAPI_BASE_URL");
        this.baseUrl = (url == null || url.isBlank() ? DEFAULT_BASE_URL : url).replaceAll("/+$", "");

        String timeout = b.timeoutMs != null ? String.valueOf(b.timeoutMs) : env("GRIDAPI_TIMEOUT_MS");
        this.timeoutMs = positive(timeout, DEFAULT_TIMEOUT_MS);
        this.maxRetries = b.maxRetries == null ? 2 : Math.max(0, b.maxRetries);
        this.retryDelayMs = b.retryDelayMs == null ? 500 : Math.max(0, b.retryDelayMs);
        this.retryWrites = Boolean.TRUE.equals(b.retryWrites);
        this.http = HttpClient.newBuilder().connectTimeout(Duration.ofMillis(timeoutMs)).build();
    }

    public GridApiClient() { this(new Builder()); }

    public GridApiClient(String apiKey) { this(builder().apiKey(apiKey)); }

    public GridApiClient(String apiKey, String baseUrl, int timeoutMs) {
        this(builder().apiKey(apiKey).baseUrl(baseUrl).timeoutMs(timeoutMs));
    }

    public static Builder builder() { return new Builder(); }

    public static final class Builder {
        private String apiKey;
        private String baseUrl;
        private Integer timeoutMs;
        private Integer maxRetries;
        private Integer retryDelayMs;
        private Boolean retryWrites;

        public Builder apiKey(String value) { this.apiKey = value; return this; }
        public Builder baseUrl(String value) { this.baseUrl = value; return this; }
        public Builder timeoutMs(int value) { this.timeoutMs = value; return this; }
        public Builder maxRetries(int value) { this.maxRetries = value; return this; }
        public Builder retryDelayMs(int value) { this.retryDelayMs = value; return this; }
        public Builder retryWrites(boolean value) { this.retryWrites = value; return this; }
        public GridApiClient build() { return new GridApiClient(this); }
    }

    // ---- Records --------------------------------------------------------

    public ListResponse<GridRecord> get(String project, String table) { return get(project, table, null); }

    public ListResponse<GridRecord> get(String project, String table, QueryOptions options) {
        return ListResponse.from(request("GET", dataPath(project, table) + queryString(options), null));
    }

    public ListResponse<GridRecord> query(String project, String table) { return get(project, table); }

    public ListResponse<GridRecord> query(String project, String table, QueryOptions options) {
        return get(project, table, options);
    }

    public GridRecord getById(String project, String table, Object id) {
        return GridRecord.from(request("GET", dataPath(project, table, id), null));
    }

    public CreateResponse<GridRecord> post(String project, String table, GridRecord record) {
        return post(project, table, record.fields());
    }

    public CreateResponse<GridRecord> post(String project, String table, Map<String, String> record) {
        return CreateResponse.from(request("POST", dataPath(project, table), record));
    }

    public CreateResponse<GridRecord> post(String project, String table, List<Map<String, String>> records) {
        return CreateResponse.from(request("POST", dataPath(project, table), records));
    }

    public UpdateResponse<GridRecord> patch(String project, String table, Object id, GridRecord changes) {
        return patch(project, table, id, changes.fields());
    }

    public UpdateResponse<GridRecord> patch(String project, String table, Object id,
                                            Map<String, String> changes) {
        return UpdateResponse.from(request("PATCH", dataPath(project, table, id), changes));
    }

    public UpdateResponse<GridRecord> put(String project, String table, Object id, GridRecord changes) {
        return put(project, table, id, changes.fields());
    }

    public UpdateResponse<GridRecord> put(String project, String table, Object id,
                                          Map<String, String> changes) {
        return UpdateResponse.from(request("PUT", dataPath(project, table, id), changes));
    }

    public DeleteResponse delete(String project, String table, Object id) {
        return DeleteResponse.from(request("DELETE", dataPath(project, table, id), null));
    }

    /** Total matching rows. Implemented as a one-row list read of meta.total. */
    public int count(String project, String table) { return count(project, table, (QueryOptions) null); }

    public int count(String project, String table, String filter) {
        return count(project, table, new QueryOptions(filter, null, 1));
    }

    public int count(String project, String table, QueryOptions options) {
        String filter = options == null ? null : options.filter();
        return get(project, table, new QueryOptions(filter, null, 1)).meta().total();
    }

    // ---- Discovery ------------------------------------------------------

    public List<ProjectInfo> listProjects() {
        Object body = request("GET", "/api/v1/me/projects", null);
        List<ProjectInfo> out = new ArrayList<>();
        for (Object p : Json.asList(Json.asMap(body).get("projects"))) out.add(ProjectInfo.from(p));
        return out;
    }

    public List<SheetInfo> listTables(String project) {
        for (ProjectInfo p : listProjects()) {
            if (matches(p, project)) return p.sheets();
        }
        throw new GridApiError.ProjectNotFoundError(
            "Project not found: " + project, GridApiError.Options.of(null, null, null));
    }

    public TableSchema getSchema(String project, String table) {
        Object body = request("GET", path("/api/v1/me/projects/{}/docs", enc(project)), null);
        for (Object endpoint : Json.asList(Json.asMap(body).get("endpoints"))) {
            String name = Json.str(Json.asMap(endpoint).get("endpoint"));
            if (name != null && name.equalsIgnoreCase(table)) return TableSchema.from(endpoint);
        }
        throw new GridApiError.TableNotFoundError(
            "Table not found: " + table, GridApiError.Options.of(null, null, null));
    }

    public AccountInfo getMe() { return AccountInfo.from(request("GET", "/api/v1/me", null)); }

    public UsageInfo getUsage() { return UsageInfo.from(request("GET", "/api/v1/me/usage", null)); }

    // ---- HTTP, retries and error mapping --------------------------------

    private Object request(String method, String path, Object body) {
        String url = baseUrl + path;
        String payload = body == null ? null : Json.write(toJsonValue(body));
        boolean safe = method.equals("GET") || method.equals("DELETE") || method.equals("PUT")
            || (method.equals("PATCH") && retryWrites);

        for (int attempt = 0; ; attempt++) {
            HttpRequest.Builder rb = HttpRequest.newBuilder(URI.create(url))
                .timeout(Duration.ofMillis(timeoutMs))
                .header("Authorization", "Bearer " + apiKey)
                .header("Accept", "application/json")
                .header("Content-Type", "application/json");
            switch (method) {
                case "GET" -> rb.GET();
                case "DELETE" -> rb.DELETE();
                default -> rb.method(method, HttpRequest.BodyPublishers.ofString(payload == null ? "" : payload));
            }

            try {
                HttpResponse<String> res = http.send(rb.build(),
                    HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
                int status = res.statusCode();
                if (status >= 200 && status < 300) {
                    return res.body().isEmpty() ? null : Json.tryParse(res.body());
                }

                Object parsed = Json.tryParse(res.body());
                String message = "GridAPI request failed with status " + status;
                String code = null;
                if (parsed instanceof Map<?, ?> m) {
                    if (Json.str(m.get("error")) != null) message = Json.str(m.get("error"));
                    else if (Json.str(m.get("message")) != null) message = Json.str(m.get("message"));
                    code = Json.str(m.get("code"));
                } else if (parsed instanceof String text && !text.isBlank()) {
                    message = text;
                }

                GridApiError error = mapHttpError(status, code, message, res.body());
                if (safe && attempt < maxRetries && error.isRetryable()) {
                    sleep(retryDelay(res.headers().firstValue("retry-after").orElse(null), attempt));
                    continue;
                }
                throw error;
            } catch (HttpTimeoutException e) {
                if (safe && attempt < maxRetries) { sleep(retryDelay(null, attempt)); continue; }
                throw new GridApiError.TimeoutError("GridAPI request timed out",
                    GridApiError.Options.of(null, null, null));
            } catch (IOException e) {
                if (safe && attempt < maxRetries) { sleep(retryDelay(null, attempt)); continue; }
                throw new GridApiError.NetworkError("GridAPI network error: " + e.getMessage(),
                    GridApiError.Options.of(null, null, null));
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                throw new GridApiError.NetworkError("GridAPI request interrupted",
                    GridApiError.Options.of(null, null, null));
            }
        }
    }

    private static GridApiError mapHttpError(int status, String code, String message, String body) {
        GridApiError.Options o = GridApiError.Options.of(status, code, body);
        if (status == 401) return new GridApiError.AuthenticationError(message, o);
        if (status == 403) return new GridApiError.AuthorizationError(message, o);
        if (status == 404) {
            String m = message == null ? "" : message.toLowerCase(Locale.ROOT);
            if ("SHEET_NOT_FOUND".equals(code) || m.contains("sheet") || m.contains("table")) {
                return new GridApiError.TableNotFoundError(message, o);
            }
            if ("NOT_FOUND".equals(code) || m.contains("record")) {
                return new GridApiError.RecordNotFoundError(message, o);
            }
            return new GridApiError.ProjectNotFoundError(message, o);
        }
        if (status == 400 || status == 422) return new GridApiError.ValidationError(message, o);
        if (status == 429) {
            return "SUBSCRIPTION_LIMIT_REACHED".equals(code)
                ? new GridApiError.QuotaExceededError(message, o)
                : new GridApiError.RateLimitError(message, o);
        }
        if (status >= 502) return new GridApiError.UpstreamProviderError(message, o);
        return new GridApiError(message, o.retryable(status >= 500));
    }

    private int retryDelay(String retryAfter, int attempt) {
        if (retryAfter != null && !retryAfter.isBlank()) {
            String v = retryAfter.trim();
            try {
                long ms = Math.max(0, Long.parseLong(v) * 1000L);
                return (int) Math.min(Integer.MAX_VALUE, ms);
            } catch (NumberFormatException ignored) { }
            try {
                Instant at = ZonedDateTime.parse(v, DateTimeFormatter.RFC_1123_DATE_TIME).toInstant();
                return (int) Math.max(0,
                    Math.min(Integer.MAX_VALUE, Duration.between(Instant.now(), at).toMillis()));
            } catch (RuntimeException ignored) { }
        }
        int jitter = retryDelayMs <= 1 ? 0 : ThreadLocalRandom.current().nextInt(Math.max(1, retryDelayMs / 2));
        return Math.min(30_000, retryDelayMs * (1 << Math.min(attempt, 20))) + jitter;
    }

    private static void sleep(int ms) {
        if (ms <= 0) return;
        try { Thread.sleep(ms); }
        catch (InterruptedException e) { Thread.currentThread().interrupt(); }
    }

    private static Object toJsonValue(Object value) {
        if (value instanceof GridRecord record) return record.fields();
        if (value instanceof Iterable<?> items) {
            List<Object> out = new ArrayList<>();
            for (Object item : items) out.add(toJsonValue(item));
            return out;
        }
        return value;
    }

    private static boolean matches(ProjectInfo project, String ref) {
        return ref.equals(project.slug()) || ref.equals(project.id()) || ref.equals(project.name());
    }

    // Endpoint templates are whole literals so contract validation can read them.
    private static String path(String template, String... parts) {
        String out = template;
        for (String part : parts) out = out.replaceFirst("\\{\\}", Matcher.quoteReplacement(part));
        return out;
    }

    private static String dataPath(String project, String table) {
        return path("/api/v1/{}/{}", enc(project), enc(table));
    }

    private static String dataPath(String project, String table, Object id) {
        return path("/api/v1/{}/{}/{}", enc(project), enc(table), enc(String.valueOf(id)));
    }

    private static String queryString(QueryOptions options) {
        if (options == null) return "";
        List<String> parts = new ArrayList<>();
        if (options.filter() != null && !options.filter().isEmpty()) {
            parts.add("filter=" + enc(options.filter()));
        }
        if (options.page() != null) parts.add("page=" + options.page());
        if (options.limit() != null) parts.add("limit=" + options.limit());
        return parts.isEmpty() ? "" : "?" + String.join("&", parts);
    }

    private static String enc(String value) { return URLEncoder.encode(value, StandardCharsets.UTF_8); }

    private static int positive(String value, int fallback) {
        if (value == null) return fallback;
        try {
            int parsed = Integer.parseInt(value.trim());
            return parsed > 0 ? parsed : fallback;
        } catch (NumberFormatException e) {
            return fallback;
        }
    }

    private static String env(String name) {
        String value = System.getenv(name);
        return value == null || value.isBlank() ? null : value;
    }
}

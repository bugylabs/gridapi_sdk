package io.gridapi;

/**
 * Base error for every GridAPI failure, mirroring the TypeScript SDK taxonomy.
 * Credentials are redacted from messages before they ever reach a log.
 */
public class GridApiError extends RuntimeException {
    private final Integer status;
    private final String code;
    private final String body;
    private final boolean retryable;

    /** Immutable error context supplied by the HTTP layer. */
    public record Options(Integer status, String code, String body, boolean retryable) {
        public static Options of(Integer status, String code, String body) {
            return new Options(status, code, body, false);
        }
        public Options retryable(boolean value) {
            return new Options(status, code, body, value);
        }
    }

    public GridApiError(String message, Options options) {
        super(redact(message));
        Options o = options == null ? Options.of(null, null, null) : options;
        this.status = o.status();
        this.code = o.code();
        this.body = o.body() == null ? null : redact(o.body());
        this.retryable = o.retryable();
    }

    public GridApiError(String message) { this(message, Options.of(null, null, null)); }

    public Integer getStatus() { return status; }
    public String getCode() { return code; }
    public String getBody() { return body; }
    public boolean isRetryable() { return retryable; }

    public static String redact(String value) {
        if (value == null) return null;
        String s = value.replaceAll("(?i)Bearer\\s+\\S+", "Bearer [REDACTED]");
        return s.replaceAll("(gp_live_|gapi_live_)[A-Za-z0-9_-]+", "$1[REDACTED]");
    }

    private static Options force(Options o, boolean value) {
        return (o == null ? Options.of(null, null, null) : o).retryable(value);
    }

    public static class AuthenticationError extends GridApiError {
        public AuthenticationError(String message, Options o) { super(message, o); }
        public AuthenticationError(String message) { super(message); }
    }
    public static class AuthorizationError extends GridApiError {
        public AuthorizationError(String message, Options o) { super(message, o); }
    }
    public static class ProjectNotFoundError extends GridApiError {
        public ProjectNotFoundError(String message, Options o) { super(message, o); }
    }
    public static class TableNotFoundError extends GridApiError {
        public TableNotFoundError(String message, Options o) { super(message, o); }
    }
    public static class RecordNotFoundError extends GridApiError {
        public RecordNotFoundError(String message, Options o) { super(message, o); }
    }
    public static class ValidationError extends GridApiError {
        public ValidationError(String message, Options o) { super(message, o); }
    }
    public static class RateLimitError extends GridApiError {
        public RateLimitError(String message, Options o) { super(message, force(o, true)); }
    }
    public static class QuotaExceededError extends GridApiError {
        public QuotaExceededError(String message, Options o) { super(message, force(o, false)); }
    }
    public static class UpstreamProviderError extends GridApiError {
        public UpstreamProviderError(String message, Options o) { super(message, force(o, true)); }
    }
    public static class NetworkError extends GridApiError {
        public NetworkError(String message, Options o) { super(message, force(o, true)); }
    }
    public static class TimeoutError extends GridApiError {
        public TimeoutError(String message, Options o) { super(message, force(o, true)); }
    }
}

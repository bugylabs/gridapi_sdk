import io.gridapi.AccountInfo;
import io.gridapi.ConfigurationException;
import io.gridapi.CreateResponse;
import io.gridapi.DeleteResponse;
import io.gridapi.GridApiClient;
import io.gridapi.GridApiError;
import io.gridapi.GridRecord;
import io.gridapi.ListResponse;
import io.gridapi.ProjectInfo;
import io.gridapi.QueryOptions;
import io.gridapi.SheetInfo;
import io.gridapi.TableSchema;
import io.gridapi.UpdateResponse;
import io.gridapi.UsageInfo;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/** Plain-Java contract test: starts the shared mock server and asserts SDK behavior. */
public final class ContractTest {
    private static final String PORT = "8792";
    private static final String TEST_KEY = "gp_live_testkey";
    private static final String PROJECT_KEY = "gapi_live_testproject";
    private static final String OTHER_KEY = "gp_live_otheraccount";
    private static String base;
    private static int checks;

    public static void main(String[] args) throws Exception {
        String root = System.getProperty("gridapi.root", ".");
        base = "http://127.0.0.1:" + PORT;
        Process server = new ProcessBuilder("node", root + "/tests/contract/mock-server.mjs", PORT)
            .redirectOutput(ProcessBuilder.Redirect.DISCARD)
            .redirectError(ProcessBuilder.Redirect.INHERIT)
            .start();
        try {
            waitForServer();
            GridApiClient client = new GridApiClient(TEST_KEY, PROJECT_KEY, base, 5_000);

            readContract(client);
            discovery(client);
            verification();
            errorMapping(client);
            redactionAndFaults();
            retries();
            crud(client);

            System.out.println("OK: " + checks + " checks passed");
        } finally {
            server.destroy();
            if (!server.waitFor(2, TimeUnit.SECONDS)) server.destroyForcibly();
        }
    }

    // ---- Read, filter, pagination, count --------------------------------

    private static void readContract(GridApiClient client) {
        ListResponse<GridRecord> all = client.get("acme", "users");
        check(all.data().size() == 3, "get returns three rows");
        check(all.meta().total() == 3, "get meta.total");
        check(all.meta().limit() == 100, "get default limit");
        check(all.meta().offset() == 0, "get default offset");

        check("Ann".equals(client.getById("acme", "users", "1").get("name")), "getById name");

        ListResponse<GridRecord> active = client.query("acme", "users",
            new QueryOptions("status:active", null, null));
        check(active.data().size() == 2, "string filter matches two rows");
        check(active.meta().total() == 2, "filtered meta.total");

        ListResponse<GridRecord> team = client.query("acme", "users",
            QueryOptions.by(Map.of("team", "growth"), null, null));
        check(team.data().size() == 2, "map filter matches two rows");

        ListResponse<GridRecord> page2 = client.get("acme", "users", new QueryOptions(null, 2, 2));
        check(page2.data().size() == 1, "page 2 of size 2 has one row");
        check("3".equals(page2.data().get(0).get("id")), "page 2 row id");
        check(page2.meta().offset() == 2 && page2.meta().limit() == 2, "pagination meta");

        check(client.count("acme", "users") == 3, "count all");
        check(client.count("acme", "users", "team:growth") == 2, "count filtered");
    }

    // ---- Discovery ------------------------------------------------------

    private static void discovery(GridApiClient client) {
        List<ProjectInfo> projects = client.listProjects();
        check(projects.size() == 2, "listProjects size");
        check("acme".equals(projects.get(0).slug()), "first project slug");

        List<SheetInfo> sheets = client.listTables("acme");
        check(sheets.size() == 2 && "users".equals(sheets.get(0).endpoint()), "listTables");
        check(sheets.get(0).allowedMethods().contains("GET"), "sheet allowedMethods");
        expectError(GridApiError.ProjectNotFoundError.class, () -> client.listTables("missing"),
            "listTables unknown project");

        TableSchema schema = client.getSchema("acme", "users");
        check("users".equals(schema.endpoint()) && "id".equals(schema.primaryKey()), "getSchema");
        check(schema.fields().size() == 2, "getSchema fields");
        expectError(GridApiError.TableNotFoundError.class, () -> client.getSchema("acme", "NOPE"),
            "getSchema unknown table");

        AccountInfo me = client.getMe();
        check(me.authenticated() && "owner@example.com".equals(me.account().email()), "getMe");
        UsageInfo usage = client.getUsage();
        check(Long.valueOf(10).equals(usage.totalApiCallsUsed()), "getUsage");
    }

    // ---- Two-key configuration and verification -------------------------

    private static void verification() {
        // Both keys are required and validated at construction.
        expectError(ConfigurationException.class, () -> new GridApiClient(null, PROJECT_KEY),
            "missing account key -> ConfigurationException");
        expectError(ConfigurationException.class, () -> new GridApiClient(TEST_KEY, null),
            "missing project key -> ConfigurationException");
        expectError(ConfigurationException.class, () -> new GridApiClient("gp_live", PROJECT_KEY),
            "malformed account key prefix -> ConfigurationException");
        expectError(ConfigurationException.class, () -> new GridApiClient(TEST_KEY, "gapi_live"),
            "malformed project key prefix -> ConfigurationException");
        expectError(ConfigurationException.class, () -> new GridApiClient("gp_live_", PROJECT_KEY),
            "empty account key suffix -> ConfigurationException");
        expectError(ConfigurationException.class, () -> new GridApiClient(TEST_KEY, "gapi_live_"),
            "empty project key suffix -> ConfigurationException");

        // Key values never appear in configuration errors.
        try {
            new GridApiClient("badprefix_secretaccount123", PROJECT_KEY);
            throw new AssertionError("FAIL: malformed account key should fail");
        } catch (ConfigurationException e) {
            checks++;
            check(!e.getMessage().contains("secretaccount123"), "account key value absent from error");
        }
        try {
            new GridApiClient(TEST_KEY, "badprefix_secretproject456");
            throw new AssertionError("FAIL: malformed project key should fail");
        } catch (ConfigurationException e) {
            checks++;
            check(!e.getMessage().contains("secretproject456"), "project key value absent from error");
        }

        GridApiClient v = new GridApiClient(TEST_KEY, PROJECT_KEY, base, 5_000);
        v.verify();
        v.verify("acme");
        v.verify("acme"); // cached: no second project lookup
        v.verify("beta");
        checks++;
        check(v.get("acme", "users").data().size() == 3, "verified client reads data");

        GridApiClient ghost = new GridApiClient(TEST_KEY, PROJECT_KEY, base, 5_000);
        expectError(GridApiError.ProjectNotFoundError.class, () -> ghost.verify("ghost"),
            "unknown project -> ProjectNotFoundError");
        expectError(GridApiError.ProjectNotFoundError.class, () -> ghost.get("ghost", "users"),
            "data method verifies project before request");

        GridApiClient unauth = new GridApiClient("gp_live_wrongkey123", PROJECT_KEY, base, 5_000);
        expectError(GridApiError.AuthenticationError.class, unauth::verify,
            "unauthenticated account -> AuthenticationError");

        GridApiClient forbidden = new GridApiClient(OTHER_KEY, PROJECT_KEY, base, 5_000);
        check(forbidden.listProjects().isEmpty(), "discovery does not trigger verification");
        expectError(GridApiError.AuthorizationError.class, () -> forbidden.verify("acme"),
            "account with no authorized projects -> AuthorizationError");
    }

    // ---- Error mapping --------------------------------------------------

    private static void errorMapping(GridApiClient client) {
        expectError(ConfigurationException.class, () -> new GridApiClient(""),
            "missing key rejected at construction");

        GridApiClient bad = new GridApiClient("gp_live_wrongkey123", PROJECT_KEY, base, 5_000);
        expectError(GridApiError.AuthenticationError.class, () -> bad.get("acme", "users"),
            "invalid key -> AuthenticationError");
        GridApiClient forbidden = new GridApiClient(OTHER_KEY, PROJECT_KEY, base, 5_000);
        expectError(GridApiError.AuthorizationError.class, () -> forbidden.get("acme", "users"),
            "other account -> AuthorizationError");

        expectError(GridApiError.ProjectNotFoundError.class, () -> client.get("missing", "users"),
            "unknown project -> ProjectNotFoundError");
        expectError(GridApiError.TableNotFoundError.class, () -> client.get("acme", "nope"),
            "unknown sheet -> TableNotFoundError");
        expectError(GridApiError.RecordNotFoundError.class, () -> client.getById("acme", "users", "999"),
            "unknown id -> RecordNotFoundError");
        expectError(GridApiError.ValidationError.class,
            () -> client.post("acme", "users", Map.of("__bad", "x")),
            "unknown column -> ValidationError");
    }

    // ---- Redaction and fault injection ----------------------------------

    private static void redactionAndFaults() {
        GridApiClient echo = new GridApiClient(TEST_KEY, PROJECT_KEY, base + "/fault/echo", 5_000);
        try {
            echo.get("acme", "users");
            throw new AssertionError("FAIL: echo fault should fail");
        } catch (GridApiError.ValidationError e) {
            checks++;
            check(e.getMessage().contains("gp_live_[REDACTED]"), "key redacted in message");
            check(!e.getMessage().contains(TEST_KEY), "raw key absent from message");
            check(e.getBody() != null && !e.getBody().contains(TEST_KEY), "raw key absent from body");
        }

        GridApiClient quota = new GridApiClient(TEST_KEY, PROJECT_KEY, base + "/fault/quota", 5_000);
        try {
            quota.get("acme", "users");
            throw new AssertionError("FAIL: quota fault should fail");
        } catch (GridApiError.QuotaExceededError e) {
            checks++;
            check("SUBSCRIPTION_LIMIT_REACHED".equals(e.getCode()), "quota code");
            check(!e.isRetryable(), "quota is terminal");
        }

        GridApiClient alwaysRate = GridApiClient.builder().apiKey(TEST_KEY).projectKey(PROJECT_KEY)
            .baseUrl(base + "/fault/always-rate").maxRetries(1).retryDelayMs(1).build();
        expectError(GridApiError.RateLimitError.class, () -> alwaysRate.get("acme", "users"),
            "persistent 429 -> RateLimitError");

        GridApiClient timeout = GridApiClient.builder().apiKey(TEST_KEY).projectKey(PROJECT_KEY)
            .baseUrl(base + "/fault/timeout").timeoutMs(150).maxRetries(0).build();
        expectError(GridApiError.TimeoutError.class, () -> timeout.get("acme", "users"),
            "no response -> TimeoutError");
    }

    // ---- Retries --------------------------------------------------------

    private static void retries() {
        // The mock's "rate" fault rejects every odd hit and passes every even
        // hit. These three calls consume hits 1..5 in order.
        GridApiClient rate = GridApiClient.builder().apiKey(TEST_KEY).projectKey(PROJECT_KEY)
            .baseUrl(base + "/fault/rate").maxRetries(2).retryDelayMs(1).build();
        check(rate.get("acme", "users").data().size() == 3, "GET retried after 429 then succeeded");

        GridApiClient patchClient = GridApiClient.builder().apiKey(TEST_KEY).projectKey(PROJECT_KEY)
            .baseUrl(base + "/fault/rate").maxRetries(2).retryDelayMs(1)
            .retryWrites(true).build();
        check(patchClient.patch("acme", "users", "1", Map.of()).updated() == 1,
            "PATCH retried when retryWrites=true");

        GridApiClient postClient = GridApiClient.builder().apiKey(TEST_KEY).projectKey(PROJECT_KEY)
            .baseUrl(base + "/fault/rate").maxRetries(2).retryDelayMs(1)
            .retryWrites(true).build();
        expectError(GridApiError.RateLimitError.class,
            () -> postClient.post("acme", "users", Map.of("name", "Never")),
            "POST never retried even with retryWrites=true");
    }

    // ---- Mutations (run last so counts above stay stable) ---------------

    private static void crud(GridApiClient client) {
        CreateResponse<GridRecord> created = client.post("acme", "users",
            Map.of("name", "Dee", "status", "active", "team", "core"));
        check(created.inserted() == 1 && created.records().size() == 1, "post inserted one row");
        String newId = created.records().get(0).get("id");
        check(newId != null && newId.startsWith("n"), "post assigned generated id");
        check("Dee".equals(created.records().get(0).get("name")), "post returned values");

        UpdateResponse<GridRecord> patched = client.patch("acme", "users", "2", Map.of("name", "Ben2"));
        check(patched.updated() == 1 && "Ben2".equals(patched.record().get("name")), "patch updated row");
        check("2".equals(patched.record().get("id")), "patch preserved id");

        UpdateResponse<GridRecord> put = client.put("acme", "users", "3",
            Map.of("name", "Cat2", "status", "active", "team", "growth"));
        check(put.updated() == 1 && "Cat2".equals(put.record().get("name")), "put replaced row");
        check("3".equals(put.record().get("id")), "put kept id");

        DeleteResponse deleted = client.delete("acme", "users", "1");
        check(deleted.deleted() == 1 && "1".equals(deleted.id()), "delete removed row");
        expectError(GridApiError.RecordNotFoundError.class, () -> client.getById("acme", "users", "1"),
            "deleted row is gone");
        check(client.count("acme", "users") == 3, "count stable after add + delete");
    }

    // ---- Harness --------------------------------------------------------

    private static void waitForServer() throws Exception {
        HttpClient http = HttpClient.newBuilder().connectTimeout(Duration.ofMillis(200)).build();
        HttpRequest request = HttpRequest.newBuilder(URI.create(base + "/api/v1/capabilities"))
            .timeout(Duration.ofMillis(300)).GET().build();
        for (int i = 0; i < 60; i++) {
            try {
                HttpResponse<String> res = http.send(request,
                    HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
                if (res.statusCode() == 200) return;
            } catch (Exception ignored) { }
            Thread.sleep(100);
        }
        throw new IllegalStateException("mock server did not start on " + base);
    }

    private static void check(boolean condition, String label) {
        checks++;
        if (!condition) throw new AssertionError("FAIL: " + label);
    }

    private static void expectError(Class<? extends Throwable> type, Runnable action, String label) {
        checks++;
        try {
            action.run();
        } catch (Throwable t) {
            if (type.isInstance(t)) return;
            throw new AssertionError("FAIL: " + label + " (got " + t.getClass().getName()
                + ": " + t.getMessage() + ")");
        }
        throw new AssertionError("FAIL: " + label + " (no error thrown)");
    }
}

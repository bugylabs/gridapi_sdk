/**
 * .NET SDK contract tests. Starts the shared mock server, exercises the SDK
 * against it, and always kills the child process in a finally block.
 */
using System.Diagnostics;
using System.Net.Sockets;
using System.Text.Json;
using GridAPI;

const string Key = "gp_live_testkey";
const string ProjectKey = "gapi_live_testproject";
const string OtherKey = "gp_live_otheraccount";
const string Base = "http://127.0.0.1:8793";
const int Port = 8793;

var passed = 0;

void Check(bool condition, string name)
{
    if (!condition) throw new Exception($"FAIL: {name}");
    passed++;
    Console.WriteLine($"PASS {name}");
}

async Task<TEx> ThrowsAsync<TEx>(Func<Task> action, string name) where TEx : Exception
{
    try
    {
        await action();
    }
    catch (TEx ex)
    {
        passed++;
        Console.WriteLine($"PASS {name}");
        return ex;
    }
    catch (Exception ex)
    {
        throw new Exception($"FAIL: {name}: expected {typeof(TEx).Name}, got {ex.GetType().Name}: {ex.Message}");
    }
    throw new Exception($"FAIL: {name}: expected {typeof(TEx).Name}, no exception thrown");
}

static string FindMockServer()
{
    var dir = new DirectoryInfo(Directory.GetCurrentDirectory());
    while (dir is not null)
    {
        var direct = Path.Combine(dir.FullName, "mock-server.mjs");
        var nested = Path.Combine(dir.FullName, "tests", "contract", "mock-server.mjs");
        if (File.Exists(direct)) return direct;
        if (File.Exists(nested)) return nested;
        dir = dir.Parent;
    }
    throw new FileNotFoundException("Could not locate tests/contract/mock-server.mjs");
}

static async Task WaitReadyAsync()
{
    for (var i = 0; i < 100; i++)
    {
        using var tcp = new TcpClient();
        try
        {
            await tcp.ConnectAsync("127.0.0.1", Port);
            return;
        }
        catch (SocketException)
        {
            await Task.Delay(100);
        }
    }
    throw new Exception("mock server did not become ready on port 8793");
}

var mockServer = FindMockServer();
var repoRoot = Path.GetFullPath(Path.Combine(Path.GetDirectoryName(mockServer)!, "..", ".."));
Process? proc = null;

try
{
    proc = Process.Start(new ProcessStartInfo("node", new[] { mockServer, Port.ToString() })
    {
        WorkingDirectory = repoRoot,
        RedirectStandardOutput = true,
        RedirectStandardError = true,
        UseShellExecute = false,
        CreateNoWindow = true,
    }) ?? throw new Exception("Failed to start node mock server");
    _ = proc.StandardOutput.ReadToEndAsync();
    _ = proc.StandardError.ReadToEndAsync();
    await WaitReadyAsync();
    Console.WriteLine("mock GridAPI ready");

    Environment.SetEnvironmentVariable("GRIDAPI_API_KEY", Key);
    Environment.SetEnvironmentVariable("GRIDAPI_PROJECT_KEY", ProjectKey);
    Environment.SetEnvironmentVariable("GRIDAPI_BASE_URL", Base);
    Environment.SetEnvironmentVariable("GRIDAPI_TIMEOUT_MS", "5000");
    using var client = new GridApiClient();

    // CRUD
    var all = await client.GetAsync("acme", "users");
    Check(all.Data.Count == 3 && all.Meta.Total == 3 && all.Meta.Limit == 100, "GET records + meta");
    var one = await client.GetByIdAsync("acme", "users", "1");
    Check(one["id"] == "1" && one["name"] == "Ann", "GET by id");

    var created = await client.PostAsync("acme", "users", new GridRecord { ["name"] = "Zed", ["status"] = "active" });
    Check(created.Inserted == 1 && created.Records.Count == 1 && created.Records[0]["name"] == "Zed", "POST create");
    var newId = created.Records[0]["id"]!;
    var patched = await client.PatchAsync("acme", "users", newId, new GridRecord { ["name"] = "Zed2" });
    Check(patched.Updated == 1 && patched.Record!["name"] == "Zed2", "PATCH update");
    var replaced = await client.PutAsync("acme", "users", newId, new GridRecord { ["id"] = newId, ["name"] = "Zed3" });
    Check(replaced.Updated == 1 && replaced.Record!["name"] == "Zed3", "PUT replace");
    var deleted = await client.DeleteAsync("acme", "users", newId);
    Check(deleted.Deleted == 1 && deleted.Id == newId, "DELETE record");

    // Filter, pagination, count
    var filtered = await client.QueryAsync("acme", "users", new QueryOptions { Filter = "status:active" });
    Check(filtered.Data.Count == 2 && filtered.Meta.Total == 2, "filter query");
    var page1 = await client.GetAsync("acme", "users", new QueryOptions { Limit = 2 });
    var page2 = await client.GetAsync("acme", "users", new QueryOptions { Limit = 2, Page = 2 });
    Check(page1.Data.Count == 2 && page2.Data.Count == 1 && page2.Data[0]["id"] == "3", "pagination page=2 limit=2");
    Check(await client.CountAsync("acme", "users", "status:active") == 2, "count via limit=1 + meta.total");

    // Configuration: both keys required, well-formed, and never echoed
    Environment.SetEnvironmentVariable("GRIDAPI_API_KEY", null);
    var missingKey = await ThrowsAsync<ConfigurationException>(() => Task.FromResult(new GridApiClient()), "missing API key => ConfigurationException");
    Environment.SetEnvironmentVariable("GRIDAPI_API_KEY", Key);
    Environment.SetEnvironmentVariable("GRIDAPI_PROJECT_KEY", null);
    var missingProject = await ThrowsAsync<ConfigurationException>(() => Task.FromResult(new GridApiClient()), "missing project key => ConfigurationException");
    Environment.SetEnvironmentVariable("GRIDAPI_PROJECT_KEY", ProjectKey);

    var malformedAccount = await ThrowsAsync<ConfigurationException>(
        () => Task.FromResult(new GridApiClient(new GridApiOptions { ApiKey = "gapi_live_wrongprefix", ProjectKey = ProjectKey, BaseUrl = Base })),
        "malformed account prefix => ConfigurationException");
    var malformedProject = await ThrowsAsync<ConfigurationException>(
        () => Task.FromResult(new GridApiClient(new GridApiOptions { ApiKey = Key, ProjectKey = "gp_live_wrongprefix", BaseUrl = Base })),
        "malformed project prefix => ConfigurationException");
    await ThrowsAsync<ConfigurationException>(
        () => Task.FromResult(new GridApiClient(new GridApiOptions { ApiKey = "gp_live_", ProjectKey = ProjectKey, BaseUrl = Base })),
        "empty account suffix => ConfigurationException");
    await ThrowsAsync<ConfigurationException>(
        () => Task.FromResult(new GridApiClient(new GridApiOptions { ApiKey = Key, ProjectKey = "gapi_live_", BaseUrl = Base })),
        "empty project suffix => ConfigurationException");
    Check(!missingKey.Message.Contains(Key) && !missingKey.Message.Contains(ProjectKey)
          && !missingProject.Message.Contains(Key) && !missingProject.Message.Contains(ProjectKey)
          && !malformedAccount.Message.Contains("gapi_live_wrongprefix")
          && !malformedProject.Message.Contains("gp_live_wrongprefix"),
          "key values absent from configuration errors");

    // Verification: cached account check plus per-project scope guard
    await client.VerifyAsync();
    await client.VerifyAsync("acme"); // slug
    await client.VerifyAsync("p1");   // id
    await client.VerifyAsync("Acme"); // name
    var afterVerify = await client.GetAsync("acme", "users");
    Check(afterVerify.Data.Count == 3, "verify succeeded and data still works");
    await ThrowsAsync<ProjectNotFoundError>(() => client.VerifyAsync("ghost"), "unknown project => ProjectNotFoundError");
    await ThrowsAsync<ProjectNotFoundError>(() => client.GetAsync("missing", "users"), "data method verifies project => ProjectNotFoundError");

    // Error mapping
    Environment.SetEnvironmentVariable("GRIDAPI_API_KEY", "gp_live_unknown");
    using var badClient = new GridApiClient();
    var authErr = await ThrowsAsync<AuthenticationError>(() => badClient.GetAsync("acme", "users"), "401 => AuthenticationError");
    Check(authErr.Status == 401 && !authErr.Message.Contains("gp_live_unknown"), "401 status exposed without key echo");
    Environment.SetEnvironmentVariable("GRIDAPI_API_KEY", Key);

    using var other = new GridApiClient(new GridApiOptions { ApiKey = OtherKey, ProjectKey = ProjectKey, BaseUrl = Base });
    await ThrowsAsync<AuthorizationError>(() => other.GetAsync("acme", "users"), "forbidden account => AuthorizationError");

    await ThrowsAsync<TableNotFoundError>(() => client.GetAsync("acme", "nope"), "404 table => TableNotFoundError");
    await ThrowsAsync<RecordNotFoundError>(() => client.GetByIdAsync("acme", "users", "999"), "404 record => RecordNotFoundError");
    await ThrowsAsync<ValidationError>(() => client.PostAsync("acme", "users", new GridRecord { ["__bad"] = "x" }), "400 => ValidationError");

    // Quota is terminal (no retries)
    using var quota = new GridApiClient(new GridApiOptions { ApiKey = Key, ProjectKey = ProjectKey, BaseUrl = Base + "/fault/quota", MaxRetries = 3, RetryDelayMs = 1 });
    var quotaErr = await ThrowsAsync<QuotaExceededError>(() => quota.GetAsync("acme", "users"), "429 quota => QuotaExceededError");
    Check(!quotaErr.Retryable && quotaErr.Code == "SUBSCRIPTION_LIMIT_REACHED", "quota terminal + code");

    // Transient rate limit is retried to success
    using var rate = new GridApiClient(new GridApiOptions { ApiKey = Key, ProjectKey = ProjectKey, BaseUrl = Base + "/fault/rate", MaxRetries = 2, RetryDelayMs = 1 });
    var rateResult = await rate.GetAsync("acme", "users");
    Check(rateResult.Data.Count == 3, "429 rate retried to success");

    // POST is never retried; on the next injected 429 it fails immediately
    using var ratePost = new GridApiClient(new GridApiOptions { ApiKey = Key, ProjectKey = ProjectKey, BaseUrl = Base + "/fault/rate", MaxRetries = 3, RetryDelayMs = 1 });
    await ThrowsAsync<RateLimitError>(() => ratePost.PostAsync("acme", "users", new GridRecord { ["name"] = "x" }), "POST never retried on 429");

    // Key redaction in error messages and bodies
    using var echo = new GridApiClient(new GridApiOptions { ApiKey = Key, ProjectKey = ProjectKey, BaseUrl = Base + "/fault/echo" });
    var echoErr = await ThrowsAsync<ValidationError>(() => echo.GetAsync("acme", "users"), "echo fault => ValidationError");
    Check(!echoErr.Message.Contains(Key) && echoErr.Message.Contains("[REDACTED]")
          && !string.IsNullOrEmpty(echoErr.Body) && !echoErr.Body.Contains(Key), "API key redacted from error message and body");

    // Timeout
    using var timeoutClient = new GridApiClient(new GridApiOptions { ApiKey = Key, ProjectKey = ProjectKey, BaseUrl = Base + "/fault/timeout", TimeoutMs = 200, MaxRetries = 0 });
    await ThrowsAsync<TimeoutError>(() => timeoutClient.GetAsync("acme", "users"), "timeout => TimeoutError");

    // Discovery
    var projects = await client.ListProjectsAsync();
    Check(projects.Count == 2 && projects[0].Slug == "acme" && projects[0].Sheets.Count == 2, "list projects");
    var tables = await client.ListTablesAsync("acme");
    Check(tables.Count == 2 && tables[0].Endpoint == "users", "list tables from /me/projects sheets");
    var schema = await client.GetSchemaAsync("acme", "users");
    Check(schema.Endpoint == "users" && schema.Fields.Any(f => f.Name == "id"), "get schema from /me/projects/{ref}/docs");
    var me = await client.GetMeAsync();
    Check(me.Authenticated && me.Account?.Email == "owner@example.com", "get me");
    var usage = await client.GetUsageAsync();
    Check(usage.TryGetValue("totalApiCallsLeft", out var left) && left.GetInt32() == 900, "get usage");

    Console.WriteLine($"ALL {passed} CHECKS PASSED");
}
catch (Exception ex)
{
    Console.Error.WriteLine(ex.Message);
    Environment.ExitCode = 1;
}
finally
{
    if (proc is not null)
    {
        try
        {
            if (!proc.HasExited) proc.Kill(entireProcessTree: true);
            proc.WaitForExit(5000);
        }
        catch
        {
            // best-effort cleanup
        }
        finally
        {
            proc.Dispose();
        }
    }
}

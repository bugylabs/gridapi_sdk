/**
 * Runnable GridAPI .NET example. Configuration comes only from the environment:
 *   GRIDAPI_API_KEY  (required)
 *   GRIDAPI_BASE_URL (optional, defaults to https://api.gridapi.in)
 *   GRIDAPI_TIMEOUT_MS (optional, defaults to 30000)
 *
 * Run:
 *   export GRIDAPI_API_KEY='...'
 *   dotnet run --project examples/dotnet
 */
using System.Text.Json;
using GridAPI;

var project = Environment.GetEnvironmentVariable("GRIDAPI_PROJECT") ?? "acme";
var table = Environment.GetEnvironmentVariable("GRIDAPI_TABLE") ?? "users";

try
{
    using var client = new GridApiClient();

    var me = await client.GetMeAsync();
    Console.WriteLine($"Authenticated: {me.Authenticated}");

    var projects = await client.ListProjectsAsync();
    foreach (var p in projects)
        Console.WriteLine($"Project {p.Slug} ({p.Role}): {string.Join(", ", p.Sheets.Select(s => s.Endpoint))}");

    var tables = await client.ListTablesAsync(project);
    Console.WriteLine($"Tables in {project}: {string.Join(", ", tables.Select(t => t.Endpoint))}");

    var schema = await client.GetSchemaAsync(project, table);
    Console.WriteLine($"Schema for {table}: {string.Join(", ", schema.Fields.Select(f => f.Name))}");

    var page = await client.GetAsync(project, table, new QueryOptions { Limit = 5 });
    Console.WriteLine($"Rows: {page.Data.Count} of {page.Meta.Total}");

    var usage = await client.GetUsageAsync();
    Console.WriteLine($"Usage: {JsonSerializer.Serialize(usage)}");
}
catch (GridApiError ex)
{
    Console.Error.WriteLine($"{ex.GetType().Name}: {ex.Message} (status {ex.Status}, code {ex.Code})");
    Environment.ExitCode = 1;
}

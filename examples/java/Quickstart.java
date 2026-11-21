import io.gridapi.CreateResponse;
import io.gridapi.GridApiClient;
import io.gridapi.GridRecord;
import io.gridapi.ListResponse;
import io.gridapi.QueryOptions;

import java.util.Map;

/**
 * Runnable GridAPI Java example. Configuration comes from the environment only:
 *   GRIDAPI_API_KEY=... GRIDAPI_BASE_URL=... java -cp <sdk-classes> Quickstart
 * Never hard-code an API key.
 */
public final class Quickstart {
    public static void main(String[] args) {
        GridApiClient client = new GridApiClient(); // reads GRIDAPI_* env vars

        ListResponse<GridRecord> users = client.get("acme", "users",
            new QueryOptions("status:active", 1, 10));
        System.out.println("total active users: " + users.meta().total());
        for (GridRecord row : users.data()) {
            System.out.println(row.get("id") + " -> " + row.get("name"));
        }

        int allUsers = client.count("acme", "users");
        System.out.println("all users: " + allUsers);

        CreateResponse<GridRecord> created = client.post("acme", "users",
            Map.of("name", "New User", "status", "active"));
        System.out.println("inserted: " + created.inserted());

        client.delete("acme", "users", created.records().get(0).get("id"));

        System.out.println("projects: " + client.listProjects().size());
        System.out.println("account: " + client.getMe().account().email());
    }
}

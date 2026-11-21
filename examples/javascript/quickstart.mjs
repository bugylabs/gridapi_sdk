import { GridApiClient } from "@gridapi/sdk";

// Never hardcode the key. Set GRIDAPI_API_KEY (and optionally GRIDAPI_BASE_URL).
const api = new GridApiClient();

const account = await api.getMe();
console.log("Authenticated:", account.authenticated);

const { data, meta } = await api.get("acme", "users", {
  filter: "status:active",
  limit: 25,
});
console.log(`Active users: ${data.length} of ${meta.total}`);

const created = await api.post("acme", "users", { name: "Ada" });
console.log("Inserted:", created.inserted);

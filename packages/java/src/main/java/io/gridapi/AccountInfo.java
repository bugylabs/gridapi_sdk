package io.gridapi;

import java.util.Map;

/** Authenticated account details from the me endpoint. */
public record AccountInfo(boolean authenticated, Account account, Map<String, Object> subscription) {
    public record Account(String id, String email, String scope) {
        static Account from(Object raw) {
            Map<String, Object> m = Json.asMap(raw);
            return new Account(Json.str(m.get("id")), Json.str(m.get("email")), Json.str(m.get("scope")));
        }
    }

    static AccountInfo from(Object raw) {
        Map<String, Object> m = Json.asMap(raw);
        Object account = m.get("account");
        return new AccountInfo(
            m.get("authenticated") instanceof Boolean b && b,
            account instanceof Map<?, ?> ? Account.from(account) : null,
            m.get("subscription") instanceof Map<?, ?> ? Json.asMap(m.get("subscription")) : Map.of()
        );
    }
}

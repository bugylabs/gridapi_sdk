package io.gridapi;

import java.util.Map;

/** Pagination metadata returned alongside a record list. */
public record ListMeta(int total, int limit, int offset, Boolean cacheHit) {
    static ListMeta from(Object raw) {
        Map<String, Object> m = Json.asMap(raw);
        return new ListMeta(
            Json.intOf(m.get("total"), 0),
            Json.intOf(m.get("limit"), 0),
            Json.intOf(m.get("offset"), 0),
            m.get("cacheHit") instanceof Boolean b ? b : null
        );
    }
}

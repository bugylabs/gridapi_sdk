package io.gridapi;

import java.util.Map;

/** Result of a PATCH or PUT: how many rows changed and the resulting row. */
public record UpdateResponse<T>(int updated, T record) {
    static UpdateResponse<GridRecord> from(Object raw) {
        Map<String, Object> m = Json.asMap(raw);
        return new UpdateResponse<>(Json.intOf(m.get("updated"), 0), GridRecord.from(m.get("record")));
    }
}

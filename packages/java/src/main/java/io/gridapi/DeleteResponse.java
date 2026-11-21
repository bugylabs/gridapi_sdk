package io.gridapi;

import java.util.Map;

/** Result of a DELETE: how many rows were removed and the matched id. */
public record DeleteResponse(int deleted, String id) {
    static DeleteResponse from(Object raw) {
        Map<String, Object> m = Json.asMap(raw);
        return new DeleteResponse(Json.intOf(m.get("deleted"), 0), Json.str(m.get("id")));
    }
}

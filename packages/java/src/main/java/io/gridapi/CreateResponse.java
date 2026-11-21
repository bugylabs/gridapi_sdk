package io.gridapi;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/** Result of a POST: how many rows were inserted and the rows themselves. */
public record CreateResponse<T>(int inserted, List<T> records) {
    static CreateResponse<GridRecord> from(Object raw) {
        Map<String, Object> m = Json.asMap(raw);
        List<GridRecord> rows = new ArrayList<>();
        for (Object o : Json.asList(m.get("records"))) rows.add(GridRecord.from(o));
        return new CreateResponse<>(Json.intOf(m.get("inserted"), 0), rows);
    }
}

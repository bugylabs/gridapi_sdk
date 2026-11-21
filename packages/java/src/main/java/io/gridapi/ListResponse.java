package io.gridapi;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/** A page of records and its metadata. */
public record ListResponse<T>(List<T> data, ListMeta meta) {
    static ListResponse<GridRecord> from(Object raw) {
        Map<String, Object> m = Json.asMap(raw);
        List<GridRecord> rows = new ArrayList<>();
        for (Object o : Json.asList(m.get("data"))) rows.add(GridRecord.from(o));
        return new ListResponse<>(rows, ListMeta.from(m.get("meta")));
    }
}
